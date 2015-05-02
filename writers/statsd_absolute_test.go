package writers

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

func newStatsdListenerUDP(t *testing.T) (*net.UDPConn, *net.UDPAddr) {
	udpAddr, err := net.ResolveUDPAddr("udp", ":1200")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	return ln, udpAddr
}

func doListenUDP(conn *net.UDPConn, ch chan string, n int) {
	for n > 0 {
		// Handle the connection in a new goroutine.
		// The loop then returns to accepting, so that
		// multiple connections may be served concurrently.
		go func(c *net.UDPConn, ch chan string) {
			buffer := make([]byte, 1024)
			size, err := c.Read(buffer)
			// size, address, err := sock.ReadFrom(buffer) <- This starts printing empty and nil values below immediatly
			if err != nil {
				fmt.Println(string(buffer), size, err)
				panic(err)
			}
			ch <- string(buffer)
		}(conn, ch)
		n--
	}
}

func jsonReadersDataForStatsdAbsoluteTest() []byte {
	jsonData := `{
    "Data": {
        "LoadAvg15m": 1.59375,
        "LoadAvg1m": 1.5537109375,
        "LoadAvg5m": 1.68798828125
    },
    "GoStruct": "LoadAvg",
    "Host": {
        "Name":"MacBook-Pro.local",
        "Tags":[]
    },
    "Interval": "1s",
    "Path": "/load-avg",
    "Tags": [ ],
    "UnixNano": 1420607791403576000
}`
	return []byte(jsonData)
}

func newWriterForStatsdAbsoluteTest() *StatsdAbsolute {
	sd := NewStatsdAbsolute()
	sd.Address = ":1200"

	readersData := make(map[string][]byte)
	readersData["/load-avg"] = jsonReadersDataForStatsdAbsoluteTest()

	sd.SetReadersDataInBytes(readersData)

	return sd
}

func TestStatsdAbsoluteRun(t *testing.T) {
	ln, _ := newStatsdListenerUDP(t)
	defer ln.Close()

	sd := newWriterForStatsdAbsoluteTest()

	ch := make(chan string, 0)

	go doListenUDP(ln, ch, len(sd.Data))

	err := sd.Run()
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			println("Warning: Statsd is not running locally.")
		} else {
			t.Errorf("Run() should never fail. Error: %v", err)
		}
	}

	for i := len(sd.Data); i > 0; i-- {
		x := <-ch

		x = strings.TrimSpace(x)
		fmt.Println(x)
	}
}
