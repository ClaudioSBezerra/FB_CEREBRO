package main

// email.go — sender SMTP próprio deste serviço (AD-7 da espinha de
// arquitetura: cada módulo implementa o seu, sem lib compartilhada entre
// repos). Espelha o formato já usado no FB_FAROL (services.SendHTMLReport)
// — multipart/alternative com texto puro e HTML — mas reimplementado aqui
// pra não acoplar este repo ao módulo Go do FB_FAROL.

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type emailConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
}

func getEmailConfig() emailConfig {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		host = "smtp.hostinger.com"
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "465"
	}
	return emailConfig{
		Host:     host,
		Port:     port,
		User:     os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
	}
}

// buildMIMEMessage monta o corpo multipart/alternative — função PURA (só
// formata string), testável sem rede nenhuma (AD-9).
func buildMIMEMessage(from string, to []string, subject, texto, htmlBody string) string {
	b := fmt.Sprintf("cerebro-jc-%d", time.Now().UnixNano())
	return fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: =?UTF-8?B?%s?=\r\n"+
		"MIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n"+
		"--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n"+
		"--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n--%s--\r\n",
		from, strings.Join(to, ", "), b64(subject), b, b, texto, b, htmlBody, b)
}

func b64(s string) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out strings.Builder
	for i := 0; i < len(s); i += 3 {
		var chunk [3]byte
		n := copy(chunk[:], s[i:min(i+3, len(s))])
		out.WriteByte(chars[chunk[0]>>2])
		out.WriteByte(chars[(chunk[0]&0x03)<<4|chunk[1]>>4])
		if n > 1 {
			out.WriteByte(chars[(chunk[1]&0x0F)<<2|chunk[2]>>6])
		} else {
			out.WriteByte('=')
		}
		if n > 2 {
			out.WriteByte(chars[chunk[2]&0x3F])
		} else {
			out.WriteByte('=')
		}
	}
	return out.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// sendHTMLReport envia o e-mail (SSL implícito, porta 465 — mesma conta
// Hostinger já em uso pelo FB_FAROL, AD-7). Falha explícita e clara se
// SMTP_USER/SMTP_PASSWORD ausentes.
func sendHTMLReport(to []string, subject, texto, htmlBody string) error {
	if len(to) == 0 {
		return fmt.Errorf("nenhum destinatário informado")
	}
	cfg := getEmailConfig()
	if cfg.User == "" || cfg.Password == "" {
		return fmt.Errorf("SMTP não configurado (SMTP_USER/SMTP_PASSWORD ausentes)")
	}
	msg := buildMIMEMessage(cfg.From, to, subject, texto, htmlBody)

	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	tlsConfig := &tls.Config{ServerName: cfg.Host}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("erro conectando SMTP: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("erro criando cliente SMTP: %w", err)
	}
	defer client.Close()

	auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("erro autenticando SMTP: %w", err)
	}
	if err := client.Mail(cfg.User); err != nil {
		return fmt.Errorf("erro MAIL FROM: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("erro RCPT TO %s: %w", addr, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("erro DATA: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("erro escrevendo corpo: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("erro fechando corpo: %w", err)
	}
	return client.Quit()
}
