package cmd

import (
	"io"
	"net"
	"time"

	"github.com/charmbracelet/log"
	"github.com/cockroachdb/errors"
	"github.com/vishvananda/netlink"
)

type StatCollector struct {
	cache map[string]int // key: IP address, value: NIC index
}

func NewStatCollector() *StatCollector {
	return &StatCollector{
		cache: make(map[string]int),
	}
}

func (sc *StatCollector) GetLinkStats(ip *net.IP) (*netlink.LinkStatistics, error) {
	if index, ok := sc.cache[ip.String()]; ok {
		link, err := netlink.LinkByIndex(index)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to get link by index")
		}
		return link.Attrs().Statistics, nil
	}

	links, err := netlink.LinkList()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list links")
	}
	for _, link := range links {
		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to list addresses")
		}
		for _, addr := range addrs {
			if addr.IP.Equal(*ip) {
				attrs := link.Attrs()
				sc.cache[ip.String()] = attrs.Index
				return attrs.Statistics, nil
			}
		}
	}
	return nil, errors.Newf("Link not found: %s", ip.String())
}

func runServer() error {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: flags.Port})
	if err != nil {
		return errors.Wrap(err, "Failed to listen")
	}

	log.Infof("Listening on %s", listener.Addr().String())

	sc := NewStatCollector()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return errors.Wrap(err, "Failed to accept")
		}
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.TCPAddr)
		logger := log.WithPrefix(conn.RemoteAddr().String())
		logger.Info("Connected")

		for {
			var req request
			if err := req.read(conn); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return errors.Wrap(err, "Failed to read request")
			}
			logger.Debugf("Received request %+v", req)

			stats, err := sc.GetLinkStats(&localAddr.IP)
			if err != nil {
				return errors.Wrap(err, "Failed to get link stats")
			}

			res := response{
				req:         req.seq,
				timestampUs: time.Now().UnixMicro(),
				txBytes:     stats.TxBytes,
				rxBytes:     stats.RxBytes,
			}

			if err := res.write(conn); err != nil {
				return errors.Wrap(err, "Failed to write response")
			}

			logger.Debugf("Sent response %+v", res)
		}
	}
}
