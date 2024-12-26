package cmd

import (
	"bytes"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/cockroachdb/errors"
	"github.com/goccy/go-yaml"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sync/errgroup"
)

type prefix net.IPNet

func (p *prefix) UnmarshalYAML(b []byte) error {
	var s string
	if err := yaml.Unmarshal(b, &s); err != nil {
		return err
	}
	if _, ipnet, err := net.ParseCIDR(s); err != nil {
		return err
	} else {
		*p = prefix(*ipnet)
	}
	return nil
}

type route struct {
	Prefix      prefix   `yaml:"prefix"`
	Gateway     net.IP   `yaml:"gateway"`
	SegmentList []net.IP `yaml:"segment_list"`
}

func (r *route) toNetlinkRoute() *netlink.Route {
	route := &netlink.Route{
		Dst:      &net.IPNet{IP: r.Prefix.IP, Mask: r.Prefix.Mask},
		Gw:       r.Gateway,
		Priority: 1,
	}
	if len(r.SegmentList) > 0 {
		route.Encap = &netlink.SEG6Encap{
			Mode:     nl.SEG6_IPTUN_MODE_ENCAP,
			Segments: r.SegmentList,
		}
	}
	return route
}

type RouteConfig struct {
	Targets []struct {
		WatchIP    net.IP        `yaml:"watch_ip"`
		Comparator string        `yaml:"comparator"`
		Threshold  string        `yaml:"threshold"`
		Interval   time.Duration `yaml:"interval"`
		IfTrue     struct {
			Routes []route `yaml:"routes"`
		} `yaml:"if_true"`
	} `yaml:"targets"`
}

func compare(comparator string, a, b float64) bool {
	switch comparator {
	case "==":
		return a == b
	case "!=":
		return a != b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	default:
		return false
	}
}

func loadRouteConfig() (*RouteConfig, error) {
	var content []byte
	if flags.ConfigPath != "" {
		// check config file exists
		if _, err := os.Stat(flags.ConfigPath); os.IsNotExist(err) {
			return nil, errors.Newf("Config file not found: %s", flags.ConfigPath)
		}

		// read config file
		file, err := os.Open(flags.ConfigPath)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to open config file")
		}
		defer file.Close()
		buf := new(bytes.Buffer)
		if _, err := io.Copy(buf, file); err != nil {
			return nil, errors.Wrap(err, "Failed to read config file")
		}
		content = buf.Bytes()
	} else if flags.ConfigYaml != "" {
		content = []byte(flags.ConfigYaml)
	} else {
		panic("Config file or YAML string must be specified")
	}

	config := RouteConfig{}
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, errors.Wrap(err, "Failed to read config file")
	}

	return &config, nil
}

func runClient() error {
	// read config file
	rc, err := loadRouteConfig()
	if err != nil {
		return err
	}
	log.Infof("Loaded config file: %s", flags.ConfigPath)
	log.Debugf("Config: %+v", rc)

	interrupt := make(chan os.Signal, 1)
	interrupted := false
	signal.Notify(interrupt, os.Interrupt)

	eg := &errgroup.Group{}
	for _, target := range rc.Targets {
		thresh, err := parseBitrate(target.Threshold)
		if err != nil {
			return errors.Wrap(err, "Failed to parse threshold")
		}
		eg.Go(func() error {
			conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{
				IP:   target.WatchIP,
				Port: flags.Port,
			})
			if err != nil {
				return errors.Wrap(err, "Failed to connect")
			}
			defer conn.Close()

			logger := log.WithPrefix(conn.RemoteAddr().String())
			logger.Info("Connected")

			// 1秒ごとにlatencyを表示
			wg := &sync.WaitGroup{}
			done := make(chan struct{})

			ema := NewEMA(flags.EmaSpan)
			var lastTxBytes, lastRxBytes uint64
			var lastTsNano int64
			lastMatched := false

			eg.Go(func() error {
				ticker := time.NewTicker(target.Interval)
				seq := uint32(0)
				for {
					select {
					case <-done:
						return nil
					case <-ticker.C:
						seq++
						req := request{seq}
						if err := req.write(conn); err != nil {
							return errors.Wrap(err, "Failed to write request")
						}
					}
				}
			})

			for !interrupted {
				conn.SetDeadline(time.Now().Add(1 * time.Second))
				var res response
				if err := res.read(conn); err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					if errors.Is(err, os.ErrDeadlineExceeded) {
						continue
					}
					return errors.Wrap(err, "Failed to read response")
				}

				logger.Debugf("Received response: %+v", res)

				if res.seq == 0 {
					// skip first response
					lastTsNano = res.tsNano
					lastTxBytes = res.txBytes
					lastRxBytes = res.rxBytes
					continue
				}

				elapsed := time.Duration(res.tsNano - lastTsNano)
				logger.Debugf("Elapsed: %s", elapsed)
				lastTsNano = res.tsNano
				bytesPerSecond := float64(res.txBytes-lastTxBytes) / float64(elapsed.Seconds())
				bytesPerSecond += float64(res.rxBytes-lastRxBytes) / float64(elapsed.Seconds())
				logger.Debugf("bps: %s", newBitrate(uint64(bytesPerSecond*8)))

				metric := ema.Update(bytesPerSecond)
				metricBps := newBitrate(uint64(metric * 8))
				logger.Debugf("EMA: %s", newBitrate(uint64(metric*8)))

				matched := compare(target.Comparator, float64(metricBps.BytesPerSecond()), float64(thresh.BytesPerSecond()))
				logger.Debugf("Threshold: %s %s %s => %t", metricBps, target.Comparator, thresh, matched)

				if matched != lastMatched {
					lastMatched = matched
					for _, route := range target.IfTrue.Routes {
						r := route.toNetlinkRoute()
						if matched {
							// add route
							if err := netlink.RouteAdd(r); err != nil {
								logger.Warnf("Failed to add route: %s: %s", err, r)
							} else {
								logger.Debugf("Added route: %s", r)
							}
						} else {
							// del route
							if err := netlink.RouteDel(r); err != nil {
								logger.Warnf("Failed to del route: %s: %s", err, r)
							} else {
								logger.Debugf("Deleted route: %s", r)
							}
						}
					}
				}

				lastTxBytes = res.txBytes
				lastRxBytes = res.rxBytes
			}

			close(done)
			wg.Wait()
			return nil
		})
	}

	<-interrupt
	interrupted = true
	log.Info("👋 Interrupted. Bye!")

	return eg.Wait()
}
