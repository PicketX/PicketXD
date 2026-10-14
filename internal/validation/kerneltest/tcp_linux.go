package kerneltest

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
)

// SendTCP emits an explicit ACK or ACK/PSH, with a valid TCP checksum, in the
// disposable client namespace. It does not depend on socket retry timing.
func (x *Topology) SendTCP(ctx context.Context, src, dst netip.Addr, sport, dport uint16, flags byte) error {
	return withNS(x.Client, func() error {
		network := "ip4:tcp"
		if src.Is6() {
			network = "ip6:tcp"
		}
		c, err := net.DialIP(network, &net.IPAddr{IP: net.IP(src.AsSlice())}, &net.IPAddr{IP: net.IP(dst.AsSlice())})
		if err != nil {
			return err
		}
		defer c.Close()
		if deadline, ok := ctx.Deadline(); ok {
			if err = c.SetDeadline(deadline); err != nil {
				return err
			}
		}
		header := make([]byte, 21)
		binary.BigEndian.PutUint16(header, sport)
		binary.BigEndian.PutUint16(header[2:], dport)
		binary.BigEndian.PutUint32(header[4:], 1)
		binary.BigEndian.PutUint32(header[8:], 1)
		header[12] = 0x50
		header[13] = flags
		binary.BigEndian.PutUint16(header[14:], 1024)
		header[20] = 'X'
		pseudo := append(append([]byte{}, src.AsSlice()...), dst.AsSlice()...)
		if src.Is4() {
			pseudo = append(pseudo, 0, 6, 0, byte(len(header)))
		} else {
			pseudo = append(pseudo, 0, 0, 0, byte(len(header)), 0, 0, 0, 6)
		}
		binary.BigEndian.PutUint16(header[16:], checksum(append(pseudo, header...)))
		_, err = c.Write(header)
		return err
	})
}
func checksum(b []byte) uint16 {
	var sum uint32
	for len(b) >= 2 {
		sum += uint32(binary.BigEndian.Uint16(b))
		b = b[2:]
	}
	if len(b) > 0 {
		sum += uint32(b[0]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}
