package realtime_test

import (
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/AndoNorth/match-flow/services/gateway-service/internal/realtime"
)

var _ = Describe("Registry", func() {
	It("delivers a published message only to clients registered for that match", func() {
		reg := realtime.NewRegistry()
		matchOneCh, unregOne := reg.Register("match-1")
		defer unregOne()
		matchTwoCh, unregTwo := reg.Register("match-2")
		defer unregTwo()

		reg.Broadcast("match-1", []byte("hello"))

		Expect(<-matchOneCh).To(Equal([]byte("hello")))
		Consistently(matchTwoCh).ShouldNot(Receive())
	})

	It("delivers every match's messages to a client registered for no specific match", func() {
		reg := realtime.NewRegistry()
		allCh, unreg := reg.Register("")
		defer unreg()

		reg.Broadcast("match-1", []byte("one"))
		reg.Broadcast("match-2", []byte("two"))

		Expect(<-allCh).To(Equal([]byte("one")))
		Expect(<-allCh).To(Equal([]byte("two")))
	})

	It("drops a message for a full client channel instead of blocking other clients", func() {
		reg := realtime.NewRegistry()
		slowCh, unregSlow := reg.Register("match-1")
		defer unregSlow()
		fastCh, unregFast := reg.Register("match-1")
		defer unregFast()

		// Fill the slow client's buffer without draining it, then send one more.
		for range cap(slowCh) + 1 {
			reg.Broadcast("match-1", []byte("x"))
		}

		// The fast client (drained here) must still have received every message
		// up to the buffer size - the slow client's full channel must not have
		// blocked delivery to it.
		for range cap(fastCh) {
			Expect(<-fastCh).To(Equal([]byte("x")))
		}
	})

	It("keeps every client registered when many Register calls race on the same registry", func() {
		reg := realtime.NewRegistry()
		const n = 50

		chs := make([]<-chan []byte, n)
		unregs := make([]func(), n)
		var wg sync.WaitGroup
		wg.Add(n)
		for i := range n {
			go func(i int) {
				defer wg.Done()
				ch, unreg := reg.Register(fmt.Sprintf("match-%d", i))
				chs[i] = ch
				unregs[i] = unreg
			}(i)
		}
		wg.Wait()
		defer func() {
			for _, unreg := range unregs {
				unreg()
			}
		}()

		for i := range n {
			reg.Broadcast(fmt.Sprintf("match-%d", i), []byte(fmt.Sprintf("payload-%d", i)))
		}
		for i := range n {
			Expect(<-chs[i]).To(Equal([]byte(fmt.Sprintf("payload-%d", i))))
		}
	})

	It("stops delivering to a client after it unregisters", func() {
		reg := realtime.NewRegistry()
		ch, unreg := reg.Register("match-1")
		unreg()

		reg.Broadcast("match-1", []byte("hello"))
		Consistently(ch).ShouldNot(Receive())
	})
})
