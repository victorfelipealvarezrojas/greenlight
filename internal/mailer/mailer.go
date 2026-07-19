package mailer

import (
	"bytes"
	"embed"
	"html/template"
	"time"

	"github.com/go-mail/mail/v2"
)

//go:embed "templates"
var templateFS embed.FS

type Mailer struct {
	dialer *mail.Dialer
	sender string
}

func New(host string, port int, username, password, sender string) Mailer {
	// Inicializa una nueva instancia de mail.Dialer con la configuración del servidor SMTP proporcionada. Nosotros
	// también configura esto para usar un tiempo de espera de 5 segundos cada vez que enviemos un correo electrónico.
	dialer := mail.NewDialer(host, port, username, password)
	dialer.Timeout = 5 * time.Second
	// Return a Mailer instance containing the dialer and sender information.
	return Mailer{
		dialer: dialer,
		sender: sender,
	}
}

func (m Mailer) Send(recipient, templateFile string, data any) error {
	// ParseFS() permite analizar el archivo de plantilla requerido desde el archivo incrustado
	tmpl, err := template.New("email").ParseFS(templateFS, "templates/"+templateFile)
	if err != nil {
		return err
	}

	// Ejecuta la plantilla denominada "subject", pasando los datos dinámicos y almacenando el
	// resultado en una variable bytes.Buffer.
	subject := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err
	}

	// Sigue el mismo patrón para ejecutar la plantilla "plainBody" y almacenar el resultado
	// en la variable PlainBody.
	plainBody := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return err
	}

	// Y lo mismo con la plantilla "htmlBody".
	htmlBody := new(bytes.Buffer)
	err = tmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err
	}

	// mail.NewMessage() inicializa una nueva instancia de mail.Message.
	// Luego usamos el metodo SetHeader() para configurar el destinatario, el remitente y el asunto del correo electrónico
	// encabezados, el metodo SetBody() para configurar el cuerpo de texto sin formato y AddAlternative()
	// metodo para configurar el cuerpo HTML. Es importante tener en cuenta que AddAlternative() debería
	// siempre será llamado *después* de SetBody().
	msg := mail.NewMessage()
	msg.SetHeader("To", recipient)
	msg.SetHeader("From", m.sender)
	msg.SetHeader("Subject", subject.String())
	msg.SetBody("text/plain", plainBody.String())
	msg.AddAlternative("text/html", htmlBody.String())
	// Llama al metodo DialAndSend() en el marcador y pasa el mensaje a enviar. esto
	// abre una conexión al servidor SMTP, envía el mensaje y luego cierra la
	// conexión. Si hay un tiempo de espera, devolverá un "dial tcp: tiempo de espera de E/S"
	//error.
	// err = m.dialer.DialAndSend(msg)

	for i := 1; i <= 3; i++ {
		err = m.dialer.DialAndSend(msg)
		// If everything worked, return nil.
		if nil == err {
			return nil
		}
		// If it didn't work, sleep for a short time and retry.
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		return err
	}
	return nil
}
