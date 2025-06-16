package conf

import (
	"encoding/base64"
	"encoding/hex"
	"net"
	"strconv"
	"strings"

	"github.com/GFW-knocker/Xray-core/common/errors"
	v2net "github.com/GFW-knocker/Xray-core/common/net"
	"github.com/GFW-knocker/Xray-core/common/protocol"
	"github.com/GFW-knocker/Xray-core/proxy/freedom"
	"google.golang.org/protobuf/proto"
)

type FreedomConfig struct {
	DomainStrategy string    `json:"domainStrategy"`
	Redirect       string    `json:"redirect"`
	UserLevel      uint32    `json:"userLevel"`
	Fragment       *Fragment `json:"fragment"`
	Noise          *Noise    `json:"noise"`
	Noises         []*Noise  `json:"noises"`
	NoiseKeepAlive uint32    `json:"noiseKeepAlive"`
	ProxyProtocol  uint32    `json:"proxyProtocol"`
}

type Fragment struct {
	Packets      string `json:"packets"`
	Length       string `json:"length"`
	Interval     string `json:"interval"`
	Host1_header string `json:"host1_header"`
	Host1_domain string `json:"host1_domain"`
	Host2_header string `json:"host2_header"`
	Host2_domain string `json:"host2_domain"`
}

type Noise struct {
	Type   string      `json:"type"`
	Packet string      `json:"packet"`
	Delay  *Int32Range `json:"delay"`
	Count  *Int32Range `json:"count"`
}

// Build implements Buildable
func (c *FreedomConfig) Build() (proto.Message, error) {
	config := new(freedom.Config)
	switch strings.ToLower(c.DomainStrategy) {
	case "asis", "":
		config.DomainStrategy = freedom.Config_AS_IS
	case "useip":
		config.DomainStrategy = freedom.Config_USE_IP
	case "useipv4":
		config.DomainStrategy = freedom.Config_USE_IP4
	case "useipv6":
		config.DomainStrategy = freedom.Config_USE_IP6
	case "useipv4v6":
		config.DomainStrategy = freedom.Config_USE_IP46
	case "useipv6v4":
		config.DomainStrategy = freedom.Config_USE_IP64
	case "forceip":
		config.DomainStrategy = freedom.Config_FORCE_IP
	case "forceipv4":
		config.DomainStrategy = freedom.Config_FORCE_IP4
	case "forceipv6":
		config.DomainStrategy = freedom.Config_FORCE_IP6
	case "forceipv4v6":
		config.DomainStrategy = freedom.Config_FORCE_IP46
	case "forceipv6v4":
		config.DomainStrategy = freedom.Config_FORCE_IP64
	default:
		return nil, errors.New("unsupported domain strategy: ", c.DomainStrategy)
	}

	if c.Fragment != nil {
		config.Fragment = new(freedom.Fragment)
		var err, err2 error

		config.Fragment.FakeHost = false

		switch strings.ToLower(c.Fragment.Packets) {
		case "tlshello":
			// TLS Hello Fragmentation (into multiple handshake messages)
			config.Fragment.PacketsFrom = 0
			config.Fragment.PacketsTo = 1
		case "fakehost":
			// fake host header with no fragmentation
			config.Fragment.PacketsFrom = 1
			config.Fragment.PacketsTo = 1
			config.Fragment.FakeHost = true
		case "":
			// TCP Segmentation (all packets)
			config.Fragment.PacketsFrom = 0
			config.Fragment.PacketsTo = 0
		default:
			// TCP Segmentation (range)
			packetsFromTo := strings.Split(c.Fragment.Packets, "-")
			if len(packetsFromTo) == 2 {
				config.Fragment.PacketsFrom, err = strconv.ParseUint(packetsFromTo[0], 10, 64)
				config.Fragment.PacketsTo, err2 = strconv.ParseUint(packetsFromTo[1], 10, 64)
			} else {
				config.Fragment.PacketsFrom, err = strconv.ParseUint(packetsFromTo[0], 10, 64)
				config.Fragment.PacketsTo = config.Fragment.PacketsFrom
			}
			if err != nil {
				return nil, errors.New("Invalid PacketsFrom").Base(err)
			}
			if err2 != nil {
				return nil, errors.New("Invalid PacketsTo").Base(err2)
			}
			if config.Fragment.PacketsFrom > config.Fragment.PacketsTo {
				config.Fragment.PacketsFrom, config.Fragment.PacketsTo = config.Fragment.PacketsTo, config.Fragment.PacketsFrom
			}
			if config.Fragment.PacketsFrom == 0 {
				return nil, errors.New("PacketsFrom can't be 0")
			}
		}

		{
			if c.Fragment.Length == "" {
				return nil, errors.New("Length can't be empty")
			}
			lengthMinMax := strings.Split(c.Fragment.Length, "-")
			if len(lengthMinMax) == 2 {
				config.Fragment.LengthMin, err = strconv.ParseUint(lengthMinMax[0], 10, 64)
				config.Fragment.LengthMax, err2 = strconv.ParseUint(lengthMinMax[1], 10, 64)
			} else {
				config.Fragment.LengthMin, err = strconv.ParseUint(lengthMinMax[0], 10, 64)
				config.Fragment.LengthMax = config.Fragment.LengthMin
			}
			if err != nil {
				return nil, errors.New("Invalid LengthMin").Base(err)
			}
			if err2 != nil {
				return nil, errors.New("Invalid LengthMax").Base(err2)
			}
			if config.Fragment.LengthMin > config.Fragment.LengthMax {
				config.Fragment.LengthMin, config.Fragment.LengthMax = config.Fragment.LengthMax, config.Fragment.LengthMin
			}
			if config.Fragment.LengthMin == 0 {
				return nil, errors.New("LengthMin can't be 0")
			}
		}

		{
			if c.Fragment.Interval == "" {
				return nil, errors.New("Interval can't be empty")
			}
			intervalMinMax := strings.Split(c.Fragment.Interval, "-")
			if len(intervalMinMax) == 2 {
				config.Fragment.IntervalMin, err = strconv.ParseUint(intervalMinMax[0], 10, 64)
				config.Fragment.IntervalMax, err2 = strconv.ParseUint(intervalMinMax[1], 10, 64)
			} else {
				config.Fragment.IntervalMin, err = strconv.ParseUint(intervalMinMax[0], 10, 64)
				config.Fragment.IntervalMax = config.Fragment.IntervalMin
			}
			if err != nil {
				return nil, errors.New("Invalid IntervalMin").Base(err)
			}
			if err2 != nil {
				return nil, errors.New("Invalid IntervalMax").Base(err2)
			}
			if config.Fragment.IntervalMin > config.Fragment.IntervalMax {
				config.Fragment.IntervalMin, config.Fragment.IntervalMax = config.Fragment.IntervalMax, config.Fragment.IntervalMin
			}
		}

		{
			if c.Fragment.Host1_header == "" {
				config.Fragment.Host1Header = "Host : "
			} else {
				config.Fragment.Host1Header = c.Fragment.Host1_header
			}

			if c.Fragment.Host1_domain == "" {
				config.Fragment.Host1Domain = "cloudflare.com"
			} else {
				config.Fragment.Host1Domain = c.Fragment.Host1_domain
			}

			if c.Fragment.Host2_header == "" {
				config.Fragment.Host2Header = "Host:   "
			} else {
				config.Fragment.Host2Header = c.Fragment.Host2_header
			}

			if c.Fragment.Host2_domain == "" {
				config.Fragment.Host2Domain = "cloudflare.com"
			} else {
				config.Fragment.Host2Domain = c.Fragment.Host2_domain
			}
		}

	}

	if c.Noise != nil {
		return nil, errors.PrintRemovedFeatureError("noise = { ... }", "noises = [ { ... } ]")
	}

	if c.Noises != nil {
		for _, n := range c.Noises {
			NConfig, err := ParseNoise(n)
			if err != nil {
				return nil, err
			}
			config.Noises = append(config.Noises, NConfig)
		}
	}

	// nosekeepalive keep repeating noise every n sec
	// if not defined in json, default is zero which is disable
	config.NoiseKeepAlive = c.NoiseKeepAlive

	config.UserLevel = c.UserLevel
	if len(c.Redirect) > 0 {
		host, portStr, err := net.SplitHostPort(c.Redirect)
		if err != nil {
			return nil, errors.New("invalid redirect address: ", c.Redirect, ": ", err).Base(err)
		}
		port, err := v2net.PortFromString(portStr)
		if err != nil {
			return nil, errors.New("invalid redirect port: ", c.Redirect, ": ", err).Base(err)
		}
		config.DestinationOverride = &freedom.DestinationOverride{
			Server: &protocol.ServerEndpoint{
				Port: uint32(port),
			},
		}

		if len(host) > 0 {
			config.DestinationOverride.Server.Address = v2net.NewIPOrDomain(v2net.ParseAddress(host))
		}
	}
	if c.ProxyProtocol > 0 && c.ProxyProtocol <= 2 {
		config.ProxyProtocol = c.ProxyProtocol
	}
	return config, nil
}

func ParseNoise(noise *Noise) (*freedom.Noise, error) {
	var err error
	NConfig := new(freedom.Noise)
	noise.Packet = strings.TrimSpace(noise.Packet)

	switch noise.Type {
	case "rand":
		min, max, err := ParseRangeString(noise.Packet)
		if err != nil {
			return nil, errors.New("invalid value for rand Length").Base(err)
		}
		NConfig.LengthMin = uint64(min)
		NConfig.LengthMax = uint64(max)
		if NConfig.LengthMin == 0 {
			return nil, errors.New("rand lengthMin or lengthMax cannot be 0")
		}

	case "str":
		// user input string
		NConfig.Packet = []byte(noise.Packet)

	case "hex":
		// user input hex
		NConfig.Packet, err = hex.DecodeString(noise.Packet)
		if err != nil {
			return nil, errors.New("Invalid hex string").Base(err)
		}

	case "base64":
		// user input base64
		NConfig.Packet, err = base64.RawURLEncoding.DecodeString(strings.NewReplacer("+", "-", "/", "_", "=", "").Replace(noise.Packet))
		if err != nil {
			return nil, errors.New("Invalid base64 string").Base(err)
		}

	default:
		return nil, errors.New("Invalid packet, only rand/str/hex/base64 are supported")
	}

	if noise.Delay != nil {
		NConfig.DelayMin = uint64(noise.Delay.From)
		NConfig.DelayMax = uint64(noise.Delay.To)
	}

	if noise.Count != nil {
		NConfig.CountMin = uint64(noise.Count.From)
		NConfig.CountMax = uint64(noise.Count.To)
	}

	return NConfig, nil
}
