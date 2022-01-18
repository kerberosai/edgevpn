// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package services_test

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"github.com/ipfs/go-log"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/mudler/edgevpn/pkg/blockchain"
	"github.com/mudler/edgevpn/pkg/logger"
	node "github.com/mudler/edgevpn/pkg/node"
	. "github.com/mudler/edgevpn/pkg/services"
)

var _ = Describe("File services", func() {
	token := node.GenerateNewConnectionData().Base64()

	logg := logger.New(log.LevelError)
	l := node.Logger(logg)

	e := node.New(node.FromBase64(true, true, token), node.WithStore(&blockchain.MemoryStore{}), l)
	e2 := node.New(node.FromBase64(true, true, token), node.WithStore(&blockchain.MemoryStore{}), l)

	Context("File sharing", func() {
		It("sends and receive files between two nodes", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			fileUUID := "test"

			le, _ := e.Ledger()

			f, err := ioutil.TempFile("", "test")
			Expect(err).ToNot(HaveOccurred())

			defer os.RemoveAll(f.Name())

			ioutil.WriteFile(f.Name(), []byte("testfile"), os.ModePerm)

			// First node expose a file
			err = ShareFile(ctx, le, e, logg, 1*time.Second, fileUUID, f.Name())
			Expect(err).ToNot(HaveOccurred())

			e.Start(ctx)
			e2.Start(ctx)

			Eventually(func() string {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				f, err := ioutil.TempFile("", "test")
				Expect(err).ToNot(HaveOccurred())

				defer os.RemoveAll(f.Name())

				ll, _ := e2.Ledger()

				ReceiveFile(ctx, ll, e2, logg, 1*time.Second, fileUUID, f.Name())
				b, _ := ioutil.ReadFile(f.Name())
				return string(b)
			}, 100*time.Second, 1*time.Second).Should(Equal("testfile"))
		})
	})
})
