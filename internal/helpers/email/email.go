package email

import (
	"MCAuth-api/internal/helpers"
	"bytes"
	"crypto/tls"
	"embed"
	"fmt"
	"html/template"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"path"
	"strings"
	"time"
)

//go:embed all:template
var templates embed.FS

const (
	dialTimeout = 15 * time.Second
	ioTimeout   = 60 * time.Second
)

type plainAuth struct{ username, password string }

func (a *plainAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a *plainAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		return nil, fmt.Errorf("unexpected server challenge: %q", fromServer)
	}
	return nil, nil
}

type loginAuth struct{ username, password string }

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte{}, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.ToLower(strings.TrimSpace(string(fromServer))) {
	case "username:":
		return []byte(a.username), nil
	case "password:":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected server challenge: %q", fromServer)
	}
}

func SendHTMLTemplate(to []string, subject, templateName string, data map[string]any) error {
	enabled := strings.TrimSpace(helpers.Getenv("SMTP_ENABLED", "false"))
	if enabled != "true" {
		return nil
	}

	host := strings.TrimSpace(helpers.Getenv("SMTP_HOST", ""))
	port := strings.TrimSpace(helpers.Getenv("SMTP_PORT", "587"))
	user := strings.TrimSpace(helpers.Getenv("SMTP_USER", ""))
	pass := strings.TrimSpace(helpers.Getenv("SMTP_PASS", ""))
	fromName := strings.TrimSpace(helpers.Getenv("SMTP_FROM_NAME", ""))
	fromAddr := strings.TrimSpace(helpers.Getenv("SMTP_FROM_ADDR", user))

	if host == "" {
		return fmt.Errorf("SMTP_HOST is not set")
	}
	if len(to) == 0 {
		return fmt.Errorf("no recipients")
	}

	from := (&mail.Address{
		Name:    fromName,
		Address: fromAddr,
	}).String()

	proto := strings.ToLower(strings.TrimSpace(helpers.Getenv("SMTP_PROTO", "")))
	if proto == "" {
		switch port {
		case "465":
			proto = "tls"
		default:
			proto = "starttls"
		}
	}

	tmplPath := path.Join("template", fmt.Sprintf("%s.html", templateName))

	tmpl, err := template.ParseFS(templates, tmplPath)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", tmplPath, err)
	}

	var body bytes.Buffer
	if err := tmpl.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject)))
	msg.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
	msg.Write(body.Bytes())

	addr := net.JoinHostPort(host, port)

	client, err := connect(proto, addr, host)
	if err != nil {
		return err
	}
	defer client.Quit()

	if err := authenticate(client, user, pass); err != nil {
		return err
	}

	return deliver(client, fromAddr, to, msg.Bytes())
}

func connect(proto, addr, host string) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: dialTimeout}

	switch proto {
	case "tls":
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: host})
		if err != nil {
			return nil, fmt.Errorf("tls dial failed: %w", err)
		}
		conn.SetDeadline(time.Now().Add(ioTimeout))
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("smtp client creation failed: %w", err)
		}
		return client, nil

	case "starttls", "none":
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("dial failed: %w", err)
		}
		conn.SetDeadline(time.Now().Add(ioTimeout))
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("smtp client creation failed: %w", err)
		}
		if proto == "starttls" {
			if ok, _ := client.Extension("STARTTLS"); !ok {
				client.Close()
				return nil, fmt.Errorf("server does not support STARTTLS")
			}
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				client.Close()
				return nil, fmt.Errorf("starttls failed: %w", err)
			}
		}
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported SMTP_PROTO: %q", proto)
	}
}

func authenticate(client *smtp.Client, user, pass string) error {
	if user == "" {
		return nil
	}

	ok, params := client.Extension("AUTH")
	if !ok {
		return nil
	}

	params = strings.ToUpper(params)

	var auth smtp.Auth
	switch {
	case strings.Contains(params, "PLAIN"):
		auth = &plainAuth{user, pass}
	case strings.Contains(params, "LOGIN"):
		auth = &loginAuth{user, pass}
	default:
		return fmt.Errorf("no supported smtp auth mechanism, server offers: %s", params)
	}

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth failed: %w", err)
	}
	return nil
}

func deliver(client *smtp.Client, fromAddr string, to []string, msg []byte) error {
	if err := client.Mail(fromAddr); err != nil {
		return fmt.Errorf("smtp MAIL FROM failed: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("smtp RCPT TO %s failed: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA failed: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp write failed: %w", err)
	}
	return w.Close()
}
