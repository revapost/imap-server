package conn

import (
	"errors"
	"fmt"
	"net/textproto"
	"regexp"
	"strings"

	"github.com/revapost/imap-server/mailstore"
	"github.com/revapost/imap-server/types"
	"github.com/revapost/imap-server/util"
)

const (
	fetchArgUID    int = 0
	fetchArgRange  int = 1
	fetchArgParams int = 2
)

var registeredFetchParams []fetchParamDefinition
var peekRE *regexp.Regexp

// ErrUnrecognisedParameter indicates that the parameter requested in a FETCH
// command is unrecognised or not implemented in this IMAP server
var ErrUnrecognisedParameter = errors.New("Unrecognised Parameter")

type fetchParamDefinition struct {
	re      *regexp.Regexp
	handler func([]string, *Conn, mailstore.Message, bool) string
}

// Register all supported fetch parameters
func init() {
	peekRE = regexp.MustCompile("\\.PEEK")
	registeredFetchParams = make([]fetchParamDefinition, 0)
	registerFetchParam("UID", fetchUID)
	registerFetchParam("FLAGS", fetchFlags)
	registerFetchParam("RFC822\\.SIZE", fetchRfcSize)
	registerFetchParam("INTERNALDATE", fetchInternalDate)
	registerFetchParam("BODY(?:\\.PEEK)?\\[HEADER\\]", fetchHeaders)
	registerFetchParam("BODY(?:\\.PEEK)?"+
		"\\[HEADER\\.FIELDS \\(([A-z\\s-]+)\\)\\]", fetchHeaderSpecificFields)
	registerFetchParam("BODY(?:\\.PEEK)?\\[TEXT\\]", fetchBody)
	registerFetchParam("BODY(?:\\.PEEK)?\\[\\]", fetchFullText)
	registerFetchParam("BODYSTRUCTURE", fetchBodyStructure)
}

func cmdFetch(args commandArgs, c *Conn) {
	if !c.assertSelected(args.ID(), readOnly) {
		return
	}

	// Fetch the messages
	seqSet, err := types.InterpretSequenceSet(args.Arg(fetchArgRange))
	if err != nil {
		c.writeResponse(args.ID(), "NO "+err.Error())
		return
	}

	searchByUID := strings.ToUpper(args.Arg(fetchArgUID)) == "UID "

	var msgs []mailstore.Message
	if searchByUID {
		msgs = c.SelectedMailbox.MessageSetByUID(seqSet)
	} else {
		msgs = c.SelectedMailbox.MessageSetBySequenceNumber(seqSet)
	}
	var fetchParamString string


	if searchByUID && !strings.Contains(args.Arg(fetchArgParams), "UID") {
		fetchParamString += "UID "
	}

	fetchParamString += args.Arg(fetchArgParams)

	for _, msg := range msgs {
		fetchParams, err := fetch(fetchParamString, c, msg)
		if err != nil {
			if err == ErrUnrecognisedParameter {
				c.writeResponse(args.ID(), "BAD Unrecognised Parameter")
				return
			}

			c.writeResponse(args.ID(), "BAD")
			return
		}

		if c.mailboxWritable == readWrite {
			msg = msg.RemoveFlags(types.FlagRecent)
			msg, err = msg.Save()
			if err != nil {
				// TODO: this error is not fatal, but should still be logged
			}
		}

		fullReply := fmt.Sprintf("%d FETCH (%s)",
			msg.SequenceNumber(),
			fetchParams)

		c.writeResponse("", fullReply)
	}

	//if searchByUID {
	//	c.writeResponse(args.ID(), "OK UID FETCH Completed")
	//} else {
		c.writeResponse(args.ID(), "OK FETCH Completed")
	//}
}

// Fetch requested params from a given message
// eg fetch("UID BODY[TEXT] RFC822.SIZE", c, message)
func fetch(params string, c *Conn, m mailstore.Message) (string, error) {
	paramList := util.SplitParams(params)

	// Prepare the list of responses
	responseParams := make([]string, 0, len(paramList))

	for _, param := range paramList {
		paramResponse, err := fetchParam(param, c, m)
		if err != nil {
			return "", err
		}
		responseParams = append(responseParams, paramResponse)
	}
	return strings.Join(responseParams, " "), nil
}

// Match a single fetch parameter and return the data
func fetchParam(param string, c *Conn, m mailstore.Message) (string, error) {
	peek := false
	if peekRE.MatchString(param) {
		peek = true
	}
	// Search through the parameter list until a parameter handler is found
	for _, element := range registeredFetchParams {
		if element.re.MatchString(param) {
			return element.handler(element.re.FindStringSubmatch(param), c, m, peek), nil
		}
	}
	return "", ErrUnrecognisedParameter
}

func registerFetchParam(regex string, handler func([]string, *Conn, mailstore.Message, bool) string) {
	newParam := fetchParamDefinition{
		re:      regexp.MustCompile(regex),
		handler: handler,
	}
	registeredFetchParams = append(registeredFetchParams, newParam)
}

// Fetch the UID of the mail message
func fetchUID(args []string, c *Conn, m mailstore.Message, peekOnly bool) string {
	return fmt.Sprintf("UID %d", m.UID())
}

func fetchFlags(args []string, c *Conn, m mailstore.Message, peekOnly bool) string {
	flags := append(m.Flags().Strings(), m.Keywords()...)
	flagList := strings.Join(flags, " ")
	return fmt.Sprintf("FLAGS (%s)", flagList)
}

func fetchRfcSize(args []string, c *Conn, m mailstore.Message, peekOnly bool) string {
	return fmt.Sprintf("RFC822.SIZE %d", m.Size())
}

func fetchInternalDate(args []string, c *Conn, m mailstore.Message, peekOnly bool) string {
	dateStr := m.InternalDate().Format(util.InternalDate)
	return fmt.Sprintf("INTERNALDATE \"%s\"", dateStr)
}

func fetchHeaders(args []string, c *Conn, m mailstore.Message, peekOnly bool) string {
	hdr := fmt.Sprintf("%s\r\n", util.MIMEHeaderToString(m.Header()))
	hdrLen := len(hdr)

	peekStr := ""
	//if peekOnly {
	//	peekStr = ".PEEK"
	//}

	return fmt.Sprintf("BODY%s[HEADER] {%d}\r\n%s", peekStr, hdrLen, hdr)
}

func fetchHeaderSpecificFields(args []string, c *Conn, m mailstore.Message, peekOnly bool) string {
	if !peekOnly {
		fmt.Printf("TODO: Peek not requested, mark all as non-recent\n")
	}
	fields := strings.Split(args[1], " ")
	hdrs := m.Header()
	requestedHeaders := make(textproto.MIMEHeader)
	replyFieldList := make([]string, len(fields))
	for i, key := range fields {
		replyFieldList[i] = "\"" + key + "\""
		// If the key exists in the headers, copy it over
		v := hdrs.Get(key)
		if v != "" {
			requestedHeaders.Add(key, v)
		}
	}
	hdr := util.MIMEHeaderToString(requestedHeaders)
	hdrLen := len(hdr)

	return fmt.Sprintf("BODY[HEADER.FIELDS (%s)] {%d}\r\n%s",
		strings.Join(replyFieldList, " "),
		hdrLen,
		hdr)

}

func fetchBody(args []string, c *Conn, m mailstore.Message, peekOnly bool) string {
	body := fmt.Sprintf("%s\r\n", m.Body())
	bodyLen := len(body)

	return fmt.Sprintf("BODY[TEXT] {%d}\r\n%s",
		bodyLen, body)
}

func fetchFullText(args []string, c *Conn, m mailstore.Message, peekOnly bool) string {
	mail := fmt.Sprintf("%s%s\r\n", util.MIMEHeaderToString(m.Header()), m.Body())
	mailLen := len(mail)

	return fmt.Sprintf("BODY[] {%d}\r\n%s",
		mailLen, mail)
}

func fetchBodyStructure(args []string, c *Conn, m mailstore.Message, peekOnly bool) string {
	body := fmt.Sprintf("%s\r\n", m.BodyStructure())
	bodyLen := len(body)

	return fmt.Sprintf("BODYSTRUCTURE {%d}\r\n%s",
		bodyLen, body)
}
