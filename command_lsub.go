package imap_server

func cmdLSub(args commandArgs, c *Conn) {
	for _, mailbox := range c.user.Mailboxes() {
		c.writeResponse("", "LSUB () \"/\" \""+mailbox.Name()+"\"")
	}
	c.writeResponse(args.Id(), "OK LSUB Completed")
}