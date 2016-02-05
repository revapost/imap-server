package conn

import (
	"strings"
	"fmt"

	"github.com/revapost/imap-server/mailstore"
	"github.com/revapost/imap-server/types"
)

const (
	searchArgUID    int = 0
	searchArgRange  int = 1
	searchArgParams int = 2
)

func cmdSearch(args commandArgs, c *Conn) {
	if !c.assertSelected(args.ID(), readOnly) {
		return
	}

	// Fetch the messages
	seqSet, err := types.InterpretSequenceSet(args.Arg(searchArgRange))
	if err != nil {
		c.writeResponse(args.ID(), "NO "+err.Error())
		return
	}

	searchByUID := strings.ToUpper(args.Arg(searchArgUID)) == "UID "

	var msgs []mailstore.Message
	if searchByUID {
		msgs = c.SelectedMailbox.MessageSetByUID(seqSet)
	} else {
		msgs = c.SelectedMailbox.MessageSetBySequenceNumber(seqSet)
	}

	searchParamString := args.Arg(searchArgParams)
	var foundUids []string
	if searchParamString == "NOT DELETED" {
		for _, msg := range msgs {
			foundUids = append(foundUids, fmt.Sprintf("%d", msg.UID()))
		}
	} else if searchParamString == "DELETED" {
		// return no message, for now, as deleted messages are already in trash
	} else {
		c.writeResponse(args.ID(), "BAD Unrecognised Parameter")
		return
	}

	fullReply := fmt.Sprintf("SEARCH %s", strings.Join(foundUids, " ") )
	c.writeResponse("*", fullReply )
	c.writeResponse(args.ID(), "OK SEARCH Completed")
}
