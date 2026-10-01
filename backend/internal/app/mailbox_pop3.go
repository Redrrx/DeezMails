package app

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"deezmails/internal/models"
	"deezmails/internal/validation"
)

// POP3 replies are normally tiny; this prevents a hostile server from making
// the buffered reader grow without bound.
const maxPOP3ResponseLineBytes = 16 * 1024

type pop3Error struct {
	Code    string
	Message string
}

func (err *pop3Error) Error() string {
	if err.Code == "" {
		return "POP3: " + err.Message
	}
	return "POP3 [" + err.Code + "]: " + err.Message
}

func (a *App) pop3Messages(ctx context.Context, account models.Account) ([]models.Email, error) {
	connection, reader, err := pop3Session(ctx, account)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	line, err := pop3Request(connection, reader, "STAT")
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid POP3 STAT response")
	}
	count, err := strconv.Atoi(parts[1])
	if err != nil || count < 0 {
		return nil, fmt.Errorf("invalid POP3 message count")
	}
	first := 1
	if count > 100 {
		first = count - 99
	}
	result := make([]models.Email, 0, count-first+1)
	for number := first; number <= count; number++ {
		uid, err := pop3Request(connection, reader, fmt.Sprintf("UIDL %d", number))
		if err != nil {
			return nil, err
		}
		uidParts := strings.Fields(uid)
		if len(uidParts) < 3 {
			continue
		}
		raw, err := pop3Multiline(connection, reader, fmt.Sprintf("RETR %d", number), a.config.MaxMessageBytes)
		if err != nil {
			return nil, err
		}
		parsed, err := mail.ReadMessage(strings.NewReader(raw))
		if err != nil {
			continue
		}
		received, _ := mail.ParseDate(parsed.Header.Get("Date"))
		result = append(result, models.Email{AccountID: account.ID, RemoteID: uidParts[2], Folder: "INBOX", From: parsed.Header.Get("From"), To: parsed.Header.Get("To"), Subject: parsed.Header.Get("Subject"), ReceivedAt: received, Raw: base64.RawURLEncoding.EncodeToString([]byte(raw)), IsRead: true})
	}
	_ = pop3Command(connection, reader, "QUIT")
	return result, nil
}

func pop3Verify(ctx context.Context, account models.Account) error {
	connection, reader, err := pop3Session(ctx, account)
	if err != nil {
		return err
	}
	defer connection.Close()
	defer pop3Command(connection, reader, "QUIT")
	return pop3Command(connection, reader, "NOOP")
}

func pop3Session(ctx context.Context, account models.Account) (net.Conn, *bufio.Reader, error) {
	if validation.HasLineBreakOrNUL(account.Email) || validation.HasLineBreakOrNUL(account.MailboxPassword) {
		return nil, nil, fmt.Errorf("mailbox credentials contain invalid line breaks")
	}
	address := net.JoinHostPort(account.IncomingHost, strconv.Itoa(account.IncomingPort))
	dialer, err := accountDialer(account)
	if err != nil {
		return nil, nil, err
	}
	connection, err := (contextDialer{dialer: dialer}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, nil, err
	}
	if err := connection.SetDeadline(mailDeadline(ctx)); err != nil {
		connection.Close()
		return nil, nil, err
	}
	if account.TLSMode == models.TLSModeImplicit {
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: account.IncomingHost, MinVersion: tls.VersionTLS12})
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			connection.Close()
			return nil, nil, err
		}
		connection = tlsConnection
	}
	reader := bufio.NewReaderSize(connection, maxPOP3ResponseLineBytes+1)
	if _, err := pop3Line(reader); err != nil {
		connection.Close()
		return nil, nil, err
	}
	if account.TLSMode == models.TLSModeStartTLS {
		if _, err := fmt.Fprint(connection, "STLS\r\n"); err != nil {
			connection.Close()
			return nil, nil, err
		}
		if _, err := pop3Line(reader); err != nil {
			connection.Close()
			if protocolError, ok := errors.AsType[*pop3Error](err); ok && protocolError.Code == "" {
				return nil, nil, &mailFailure{Code: models.MailErrorTLS, Action: models.MailErrorActionEditAccount, Message: "POP3 server does not accept STARTTLS. Edit the account security mode.", Cause: err}
			}
			return nil, nil, err
		}
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: account.IncomingHost, MinVersion: tls.VersionTLS12})
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			connection.Close()
			return nil, nil, err
		}
		connection = tlsConnection
		reader = bufio.NewReaderSize(connection, maxPOP3ResponseLineBytes+1)
	}
	for _, command := range []string{"USER " + account.Email, "PASS " + account.MailboxPassword} {
		if err := pop3Command(connection, reader, command); err != nil {
			if protocolError, ok := errors.AsType[*pop3Error](err); ok && protocolError.Code == "" {
				protocolError.Code = "AUTH"
			}
			connection.Close()
			return nil, nil, err
		}
	}
	return connection, reader, nil
}

func pop3Request(connection net.Conn, reader *bufio.Reader, command string) (string, error) {
	if _, err := fmt.Fprint(connection, command+"\r\n"); err != nil {
		return "", err
	}
	return pop3Line(reader)
}
func pop3Command(connection net.Conn, reader *bufio.Reader, command string) error {
	_, err := pop3Request(connection, reader, command)
	return err
}
func pop3Line(reader *bufio.Reader) (string, error) {
	line, err := pop3ReadLine(reader)
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "+OK") {
		return line, nil
	}
	message := strings.TrimSpace(strings.TrimPrefix(line, "-ERR"))
	code := ""
	if strings.HasPrefix(message, "[") {
		if end := strings.IndexByte(message, ']'); end > 1 {
			if fields := strings.Fields(message[1:end]); len(fields) != 0 {
				code = strings.ToUpper(fields[0])
			}
			message = strings.TrimSpace(message[end+1:])
		}
	}
	return "", &pop3Error{Code: code, Message: message}
}
func pop3Multiline(connection net.Conn, reader *bufio.Reader, command string, maxMessageBytes int64) (string, error) {
	if _, err := pop3Request(connection, reader, command); err != nil {
		return "", err
	}
	var output strings.Builder
	for {
		line, err := pop3ReadLine(reader)
		if err != nil {
			return "", err
		}
		if line == "." {
			break
		}
		if strings.HasPrefix(line, "..") {
			line = line[1:]
		}
		output.WriteString(line)
		output.WriteString("\r\n")
		if int64(output.Len()) > maxMessageBytes {
			return "", fmt.Errorf("POP3 message exceeds the %d byte limit: %w", maxMessageBytes, &http.MaxBytesError{Limit: maxMessageBytes})
		}
	}
	return output.String(), nil
}

func pop3ReadLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return "", fmt.Errorf("POP3 response line exceeds the %d byte limit: %w", maxPOP3ResponseLineBytes, &http.MaxBytesError{Limit: maxPOP3ResponseLineBytes})
	}
	if err != nil {
		return "", err
	}
	if len(line) > maxPOP3ResponseLineBytes {
		return "", fmt.Errorf("POP3 response line exceeds the %d byte limit: %w", maxPOP3ResponseLineBytes, &http.MaxBytesError{Limit: maxPOP3ResponseLineBytes})
	}
	return strings.TrimRight(string(line), "\r\n"), nil
}
