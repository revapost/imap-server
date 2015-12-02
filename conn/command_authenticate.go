package conn

import (
	"encoding/base64"
	"regexp"
	"fmt"
)

// Handles PLAIN text AUTHENTICATE command
func cmdAuthPlain(args commandArgs, c *Conn) {
	// Compile login regex
	loginRE := regexp.MustCompile("(?:[\\S]+)?\x00([A-z0-9@._-]+)\x00([\\S]+)")

	// Tell client to go ahead
	c.writeResponse("+", "")

	// Wait for client to send auth details
	ok := c.RwcScanner.Scan()
	if !ok {
		return
	}
	authDetails := c.RwcScanner.Text()
	fmt.Fprintf(c.Transcript, "C: %s", authDetails)

	data, err := base64.StdEncoding.DecodeString(authDetails)
	if err != nil {
		c.writeResponse("", "BAD Invalid auth details")
		return
	}
	fmt.Fprintf(c.Transcript, " (decoded) %s\n", data)
	match := loginRE.FindSubmatch(data)
	if len(match) != 3 {
		c.writeResponse(args.ID(), "NO Incorrect username/password")
		return
	}
	c.User, err = c.Mailstore.Authenticate(string(match[1]), string(match[2]))
	if err != nil {
		c.writeResponse(args.ID(), "NO Incorrect username/password")
		return
	}
	c.SetState(StateAuthenticated)
	c.writeResponse(args.ID(), "OK Authenticated")
}
