# Connection sweeper — data gathering demo

## Setup

Needs: a built `skupper-router`, plus the `a.conf`, `b.conf`, and `random_traffic_sim.py` files below.

Create/modify these three files in order to run the tests.

---

`a.conf` — in `<skupper-router>/build/`

```conf
router {
   mode: interior
   id: A
}

listener {
   port: amqp
   authenticatePeer: no
   saslMechanisms: ANONYMOUS
}

listener {
   port: 10000
   authenticatePeer: no
   saslMechanisms: ANONYMOUS
   role: inter-router
}

tcpListener {
   port: 9090
   address: tcp-echo
   name: tcp-echo-listener
}
```

---

`b.conf` — in `<skupper-router>/build/`

```conf
router {
    mode: interior
    id: B
}
listener {
   host: 127.0.0.1
    port: 5673
    role: normal
}
connector {
    host: 127.0.0.1
    port: 10000
    role: inter-router
}
tcpConnector {
    address: tcp-echo
    host: 127.0.0.1
    port: 9091
    name: tcp-echo-connector
}
```

---

`random_traffic_sim.py` — in `<skupper-router>/scripts/`

```python
#!/usr/bin/env python3
"""
Opens random TCP connections to a router's TCP adaptor, each sending a small payload at a random interval.
"""

import argparse
import random
import signal
import socket
import threading
import time
from datetime import datetime


def log(msg: str) -> None:
    ts = datetime.now().strftime("%H:%M:%S")
    print(f"[{ts}] {msg}", flush=True)


def _run_backend(port: int) -> None:
    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind(("0.0.0.0", port))
    srv.listen(256)
    while True:
        try:
            conn, _ = srv.accept()
            threading.Thread(target=_backend_echo, args=(conn,), daemon=True).start()
        except OSError:
            break


def _backend_echo(conn: socket.socket) -> None:
    try:
        while True:
            data = conn.recv(4096)
            if not data:
                return
            conn.sendall(data)
    except OSError:
        pass
    finally:
        conn.close()


def _connection_worker(idx: int, host: str, port: int, min_interval: int, max_interval: int,
                        stop: threading.Event) -> None:
    """Holds one TCP connection open for the whole run, sending a payload and
    reading the echo back at an independently randomized interval."""
    sock = None
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.connect((host, port))
        sock.sendall(f"conn-{idx} open\n".encode())
        sock.settimeout(5.0)
        try:
            sock.recv(4096)
        except socket.timeout:
            pass

        while not stop.is_set():
            interval = random.randint(min_interval, max_interval)
            if stop.wait(interval):
                break
            try:
                payload = f"conn-{idx} ping {int(time.time())}\n".encode()
                sock.sendall(payload)
                sock.recv(4096)
            except OSError as e:
                log(f"conn-{idx} error, exiting: {e}")
                return
    except OSError as e:
        log(f"conn-{idx} failed to connect: {e}")
    finally:
        if sock:
            sock.close()


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="random_traffic_sim.py",
        description="Steady TCP connections sending data at random intervals, for sweeper testing.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument("--host", default="127.0.0.1", help="Router host (default: 127.0.0.1)")
    parser.add_argument("--port", type=int, default=9090, help="Router TCP adaptor port (default: 9090)")
    parser.add_argument("--connections", type=int, default=20, help="Number of connections to open (default: 20)")
    parser.add_argument("--min-interval", type=int, default=30,
                         help="Minimum seconds between sends on a connection (default: 30)")
    parser.add_argument("--max-interval", type=int, default=500,
                         help="Maximum seconds between sends on a connection (default: 500)")
    parser.add_argument("--backend-port", type=int, default=None,
                         help="Port for this script's own echo backend (default: --port + 1)")
    args = parser.parse_args()

    backend_port = args.backend_port or (args.port + 1)
    stop = threading.Event()

    def _on_sigint(_s, _f):
        log("Interrupted — shutting down ...")
        stop.set()

    signal.signal(signal.SIGINT, _on_sigint)

    threading.Thread(target=_run_backend, args=(backend_port,), daemon=True).start()
    time.sleep(0.3)
    log(f"Backend echoing on :{backend_port}")

    log(f"Opening {args.connections} connection(s) to {args.host}:{args.port}, "
        f"each sending every {args.min_interval}-{args.max_interval}s ...")
    threads = []
    for i in range(args.connections):
        t = threading.Thread(
            target=_connection_worker,
            args=(i, args.host, args.port, args.min_interval, args.max_interval, stop),
            daemon=True,
        )
        t.start()
        threads.append(t)
        time.sleep(0.05)

    log("All connections open. Running until Ctrl+C.")
    stop.wait()

    for t in threads:
        t.join(timeout=2)
    log("Exited.")


if __name__ == "__main__":
    main()
```

---

Copy-paste, once per terminal (need 4 terminals):

```bash
export SKUPPER_ROUTER=~/skupper-router   # adjust if your checkout lives elsewhere
export SKUPPER_REPO=~/skupper
[ -f "$SKUPPER_ROUTER/build/config.sh" ] || echo "!! not found — fix SKUPPER_ROUTER above"
source "$SKUPPER_ROUTER/build/config.sh"
cd "$SKUPPER_REPO"
```

Structured as client -> Router A -> Router B -> echo backend

## Run it

**Terminal 1 — Router A:**
```bash
"$SKUPPER_ROUTER/build/router/skrouterd" -c "$SKUPPER_ROUTER/build/a.conf"
```

**Terminal 2 — Router B:**
```bash
"$SKUPPER_ROUTER/build/router/skrouterd" -c "$SKUPPER_ROUTER/build/b.conf"
```

**Terminal 3 — traffic** (starts its own echo backend on `:9091`):
```bash
python3 "$SKUPPER_ROUTER/scripts/random_traffic_sim.py" --connections 6 --min-interval 30 --max-interval 300
```

Wait for `All connections open.`, then run the tests below in a fourth terminal.

---

## Test 1 — the `in` side

```bash
go run ./internal/cmd/skupper/debug/sweeper/gatherdump --url amqp://127.0.0.1:5672
```

```
DIR  HOST                      LOCALSOCKET               UPTIME(s)  LASTDLV(s)
in   127.0.0.1:53484           127.0.0.1:9090            123        123
in   127.0.0.1:53498           127.0.0.1:9090            123        123
in   127.0.0.1:53506           127.0.0.1:9090            123        123
in   127.0.0.1:53514           127.0.0.1:9090            123        123
in   127.0.0.1:53528           127.0.0.1:9090            123        123
in   127.0.0.1:53538           127.0.0.1:9090            123        123
```

`HOST` is unique per connection. `LOCALSOCKET` is the same on every row cause it's the listener.

The two `sockets by ...` sections below the table are the same kernel sockets indexed by each end — `in` connections are matched by their peer address, `out` connections by their local address.


## Test 2 — the `out` side

Same command, Router B:

```bash
go run ./internal/cmd/skupper/debug/sweeper/gatherdump --url amqp://127.0.0.1:5673
```

```
DIR  HOST                      LOCALSOCKET               UPTIME(s)  LASTDLV(s)
out  127.0.0.1:9091            127.0.0.1:54944           133        133
out  127.0.0.1:9091            127.0.0.1:54948           133        133
out  127.0.0.1:9091            127.0.0.1:54956           133        133
out  127.0.0.1:9091            127.0.0.1:54960           133        133
out  127.0.0.1:9091            127.0.0.1:54972           133        133
out  127.0.0.1:9091            127.0.0.1:54976           133        133
```

## Clean up
```bash
pkill -f 'skrouterd -c .*a\.conf'
pkill -f 'skrouterd -c .*b\.conf'
pkill -f random_traffic_sim.py
```
