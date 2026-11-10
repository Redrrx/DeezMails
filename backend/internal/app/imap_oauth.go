package app


type imapOAuthClient struct {
	email string
	token string
}

func (c *imapOAuthClient) Start() (string, []byte, error) {
	return "XOAUTH2", []byte("user=" + c.email + "\x01auth=Bearer " + c.token + "\x01\x01"), nil
}

func (c *imapOAuthClient) Next(_ []byte) ([]byte, error) {
	return []byte{}, nil
}
