package smtp_test

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email"
	"github.com/Spicy-Bush/fider-tarkov-community/app/services/email/smtp"
)

var sendSMTP = smtp.Send

func TestDeliveryAcknowledgementAndRejection(t *testing.T) {
	for _, scenario := range []string{"recipient-rejected", "temporary-rejection", "quit-disconnect"} {
		t.Run(scenario, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan error, 1)
			go func() {
				connection, err := listener.Accept()
				if err != nil {
					done <- err
					return
				}
				defer connection.Close()
				connection.SetDeadline(time.Now().Add(5 * time.Second))
				fmt.Fprint(connection, "220 local test\r\n")
				reader := bufio.NewReader(connection)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						done <- err
						return
					}
					switch {
					case strings.HasPrefix(line, "RCPT") && scenario != "quit-disconnect":
						if scenario == "recipient-rejected" {
							fmt.Fprint(connection, "550 mailbox unavailable\r\n")
						} else {
							fmt.Fprint(connection, "450 mailbox temporarily unavailable\r\n")
						}
						done <- nil
						return
					case strings.HasPrefix(line, "DATA"):
						fmt.Fprint(connection, "354 send data\r\n")
						for {
							line, err = reader.ReadString('\n')
							if err != nil || line == ".\r\n" {
								break
							}
						}
						fmt.Fprint(connection, "250 accepted\r\n")
					case strings.HasPrefix(line, "QUIT"):
						done <- nil
						return
					default:
						fmt.Fprint(connection, "250 ok\r\n")
					}
				}
			}()
			err = sendSMTP("localhost", listener.Addr().String(), false, nil, "from@example.com", []string{"to@example.com"}, []byte("Subject: Test\r\n\r\nBody"))
			if scenario == "quit-disconnect" && err != nil {
				t.Fatalf("accepted mail became a failure: %v", err)
			}
			if scenario != "quit-disconnect" && err == nil {
				t.Fatal("rejected mail reported success")
			}
			if email.IsRecipientRejected(err) != (scenario == "recipient-rejected") {
				t.Fatalf("incorrect rejection classification: %v", err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}
