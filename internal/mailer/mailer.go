package mailer

import (
	"bytes"
	"embed"
	"time"

	ht "html/template"
	tt "text/template"

	"github.com/wneessen/go-mail"
)

// `//go:embed <path>` indicates that
// we want to store the contents of ./templates directory in the templateFS embedded file system variable

//go:embed "templates"
var tempateFS embed.FS

// *mail.Client instance that connects to a SMTP server
// sender info (name and address) you want the email to be from ie "Ijumaa Hatari <ijumaa@example.com>"
type Mailer struct {
	client *mail.Client
	sender string
}

func New(host string, port int, username, password, sender string) (*Mailer, error) {
	// mail.Dialer instance
	client, err := mail.NewClient(
		host,
		mail.WithSMTPAuth(mail.SMTPAuthLogin),
		mail.WithPort(port),
		mail.WithUsername(username),
		mail.WithPassword(password),
		mail.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, err
	}

	mailer := &Mailer{
		client: client,
		sender: sender,
	}

	return mailer, nil
}

// takes in recipient's email as the first parameter
// followed by template
// and dynamic data for the template
func (m *Mailer) Send(recipient string, templateFile string, data any) error {
	textTmpl, err := tt.New("").ParseFS(tempateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}

	subject := new(bytes.Buffer)
	err = textTmpl.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err
	}

	plainBody := new(bytes.Buffer)
	err = textTmpl.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return err
	}

	htmlTmpl, err := ht.New("").ParseFS(tempateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}

	htmlBody := new(bytes.Buffer)
	err = htmlTmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err
	}

	msg := mail.NewMsg()
	err = msg.To(recipient)
	if err != nil {
		return err
	}

	err = msg.From(m.sender)
	if err != nil {
		return err
	}

	msg.Subject(subject.String())
	msg.SetBodyString(mail.TypeTextPlain, plainBody.String())
	msg.AddAlternativeString(mail.TypeTextHTML, htmlBody.String())

	// try sending the email up to three times  before aborting
	// returning the error
	for i := 1; i <= 3; i++ {
		// open a connection to the smtp server and send the message
		// then close the connection
		err = m.client.DialAndSend(msg)
		if err == nil {
			return nil
		}

		if i != 3 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return err
}
