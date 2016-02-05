package conn_test

import (
	"github.com/revapost/imap-server/conn"
	. "github.com/onsi/ginkgo"
)

var _ = Describe("LSUB Command", func() {
	Context("When logged in", func() {
		BeforeEach(func() {
			tConn.SetState(conn.StateAuthenticated)
			tConn.User = mStore.User
		})

		PIt("should (implement test)", func() {
		})
	})

	Context("When not logged in", func() {
		BeforeEach(func() {
			tConn.SetState(conn.StateNotAuthenticated)
		})

		PIt("should give an error", func() {
		})
	})
})
