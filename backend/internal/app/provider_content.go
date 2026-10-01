package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"deezmails/internal/models"

	_ "github.com/emersion/go-message/charset"
	messagemail "github.com/emersion/go-message/mail"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/oauth2"
)

const maxMIMEParts = 1000

var htmlTextIgnoredElements = map[atom.Atom]struct{}{
	atom.Head: {}, atom.Noscript: {}, atom.Script: {}, atom.Style: {}, atom.Svg: {},
}

var htmlTextLineBreakElements = map[atom.Atom]struct{}{
	atom.Address: {}, atom.Article: {}, atom.Aside: {}, atom.Blockquote: {},
	atom.Br: {}, atom.Dd: {}, atom.Div: {}, atom.Dl: {}, atom.Dt: {},
	atom.Footer: {}, atom.H1: {}, atom.H2: {}, atom.H3: {}, atom.H4: {},
	atom.H5: {}, atom.H6: {}, atom.Header: {}, atom.Hr: {}, atom.Li: {},
	atom.Main: {}, atom.Ol: {}, atom.P: {}, atom.Pre: {}, atom.Section: {},
	atom.Table: {}, atom.Tbody: {}, atom.Td: {}, atom.Tfoot: {}, atom.Th: {},
	atom.Thead: {}, atom.Tr: {}, atom.Ul: {},
}

func (a *App) fetchRawMessage(ctx context.Context, account models.Account, token *oauth2.Token, id string) ([]byte, error) {
	endpointID := url.PathEscape(id)
	switch account.Provider {
	case models.ProviderGmail:
		service, err := a.gmailService(ctx, account, token)
		if err != nil {
			return nil, err
		}
		message, err := service.Users.Messages.Get("me", id).Format("raw").Fields("raw").Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(message.Raw, "="))
		if err != nil {
			return nil, fmt.Errorf("decode Gmail raw message: %w", err)
		}
		if len(raw) == 0 {
			return nil, errors.New("gmail returned an empty raw message")
		}
		return raw, nil
	case models.ProviderMicrosoft:
		client, err := providerClient(account, token, a.config.MaxMessageBytes)
		if err != nil {
			return nil, err
		}
		raw, err := a.microsoftGet(ctx, client, "https://graph.microsoft.com/v1.0/me/messages/"+endpointID+"/$value")
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			return nil, errors.New("microsoft returned an empty raw message")
		}
		return raw, nil
	default:
		return nil, fmt.Errorf("provider %q does not expose OAuth raw messages", account.Provider)
	}
}

func extractMessageText(raw []byte, maxBytes int64) (string, error) {
	reader, err := messagemail.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("parse MIME message: %w", err)
	}
	defer reader.Close()

	var htmlFallback string
	for parts := 0; ; {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return htmlFallback, nil
		}
		if err != nil {
			return "", fmt.Errorf("read MIME part: %w", err)
		}
		parts++
		if parts > maxMIMEParts {
			return "", fmt.Errorf("MIME message exceeds the %d part limit", maxMIMEParts)
		}
		header, ok := part.Header.(*messagemail.InlineHeader)
		if !ok {
			continue
		}
		mediaType, _, err := header.ContentType()
		if err != nil {
			return "", fmt.Errorf("parse MIME content type: %w", err)
		}
		mediaType = strings.ToLower(mediaType)
		if mediaType != "text/plain" && mediaType != "text/html" {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(part.Body, maxBytes+1))
		if err != nil {
			return "", fmt.Errorf("read MIME body: %w", err)
		}
		if int64(len(content)) > maxBytes {
			return "", fmt.Errorf("MIME body exceeds the %d byte limit", maxBytes)
		}
		if mediaType == "text/plain" {
			plain := strings.TrimSpace(strings.ToValidUTF8(string(content), "�"))
			if plain != "" {
				return plain, nil
			}
			continue
		}
		if htmlFallback == "" {
			htmlFallback, err = plainTextFromHTML(string(content))
			if err != nil {
				return "", err
			}
		}
	}
}

func plainTextFromHTML(source string) (string, error) {
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	var output strings.Builder
	var skippedTag atom.Atom
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			if err := tokenizer.Err(); !errors.Is(err, io.EOF) {
				return "", fmt.Errorf("parse HTML body: %w", err)
			}
			break
		}
		token := tokenizer.Token()
		if skippedTag != 0 {
			if tokenType == html.EndTagToken && token.DataAtom == skippedTag {
				skippedTag = 0
			}
			continue
		}
		switch tokenType {
		case html.StartTagToken:
			if _, ignored := htmlTextIgnoredElements[token.DataAtom]; ignored {
				skippedTag = token.DataAtom
				continue
			}
			if _, breaksLine := htmlTextLineBreakElements[token.DataAtom]; breaksLine {
				output.WriteByte('\n')
			}
		case html.EndTagToken, html.SelfClosingTagToken:
			if _, breaksLine := htmlTextLineBreakElements[token.DataAtom]; breaksLine {
				output.WriteByte('\n')
			}
		case html.TextToken:
			output.WriteString(token.Data)
		}
	}

	lines := make([]string, 0)
	for _, line := range strings.Split(output.String(), "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n"), nil
}
