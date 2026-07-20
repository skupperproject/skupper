package sweeper

import (
	"strconv"
	"strings"
)

// inetDiagScript is a python stand-in for `ss -tin`, used when ss is
// not available like when the router image ships python3 but no iproute2. It has to
// run where the router runs (sockets are per network namespace), so it can
// only use what the router image provides.
//
//
//	<local ip:port> <peer ip:port> <lastrcv ms> <lastsnd ms>
//
// TODO: IPv4 only; extend for IPv6.

const inetDiagScript = `
import socket, struct

NETLINK_SOCK_DIAG = 4
SOCK_DIAG_BY_FAMILY = 20
NLM_F_REQUEST_DUMP = 0x301
NLMSG_DONE = 3
NLMSG_ERROR = 2
INET_DIAG_INFO = 2

s = socket.socket(socket.AF_NETLINK, socket.SOCK_RAW, NETLINK_SOCK_DIAG)
# inet_diag_req_v2: family, protocol, ext (request tcp_info), pad,
# states bitmask (established only), zeroed socket id
req = struct.pack("=BBBBI48s", socket.AF_INET, socket.IPPROTO_TCP,
                  1 << (INET_DIAG_INFO - 1), 0, 1 << 1, b"")
s.send(struct.pack("=IHHII", 16 + len(req), SOCK_DIAG_BY_FAMILY,
                   NLM_F_REQUEST_DUMP, 1, 0) + req)

done = False
while not done:
    data = s.recv(1 << 20)
    off = 0
    while off + 16 <= len(data):
        ln, typ = struct.unpack_from("=IH", data, off)
        if ln < 16 or typ in (NLMSG_DONE, NLMSG_ERROR):
            done = True
            break
        # inet_diag_msg: sport/dport are big-endian at +4/+6, src at +8,
        # dst at +24 (IPv4 uses the first 4 of 16 address bytes)
        sport, dport = struct.unpack_from(">HH", data, off + 20)
        src = socket.inet_ntoa(data[off + 24:off + 28])
        dst = socket.inet_ntoa(data[off + 40:off + 44])
        lastrcv = lastsnd = 0
        aoff = off + 16 + 72  # rtattrs follow the 72-byte inet_diag_msg
        while aoff + 4 <= off + ln:
            alen, atype = struct.unpack_from("=HH", data, aoff)
            if alen < 4:
                break
            if atype == INET_DIAG_INFO and alen >= 4 + 56:
                # tcp_info: tcpi_last_data_sent at +44, tcpi_last_data_recv at +52
                lastsnd = struct.unpack_from("=I", data, aoff + 4 + 44)[0]
                lastrcv = struct.unpack_from("=I", data, aoff + 4 + 52)[0]
            aoff += (alen + 3) & ~3
        print("%s:%d %s:%d %d %d" % (src, sport, dst, dport, lastrcv, lastsnd))
        off += (ln + 3) & ~3
`

// socketsFromDiagOutput parses inetDiagScript's output into the same two
// maps socketsFromSS produces.
func socketsFromDiagOutput(out []byte) (byPeer, byLocal map[string]socketInfo) {
	byPeer = map[string]socketInfo{}
	byLocal = map[string]socketInfo{}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 4 {
			continue
		}
		rcv, err1 := strconv.Atoi(f[2])
		snd, err2 := strconv.Atoi(f[3])
		if err1 != nil || err2 != nil {
			continue
		}
		sock := socketInfo{LastRcvMs: rcv, LastSndMs: snd}
		byLocal[f[0]] = sock
		byPeer[f[1]] = sock
	}
	return byPeer, byLocal
}
