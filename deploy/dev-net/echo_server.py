#!/usr/bin/env python3
"""Minimal TCP + UDP echo for ops-mcp Connector smoke tests."""

from __future__ import annotations

import socket
import threading

TCP_PORT = 9090
UDP_PORT = 9091
BUF = 65536


def tcp_echo() -> None:
    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind(("0.0.0.0", TCP_PORT))
    srv.listen(32)
    while True:
        conn, _ = srv.accept()
        threading.Thread(target=_tcp_handle, args=(conn,), daemon=True).start()


def _tcp_handle(conn: socket.socket) -> None:
    try:
        chunks: list[bytes] = []
        while True:
            data = conn.recv(BUF)
            if not data:
                break
            chunks.append(data)
        payload = b"".join(chunks)
        if payload:
            conn.sendall(payload)
    finally:
        conn.close()


def udp_echo() -> None:
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind(("0.0.0.0", UDP_PORT))
    while True:
        data, addr = sock.recvfrom(BUF)
        if data:
            sock.sendto(data, addr)


def main() -> None:
    threading.Thread(target=tcp_echo, daemon=True).start()
    print(f"tcp echo on :{TCP_PORT}", flush=True)
    print(f"udp echo on :{UDP_PORT}", flush=True)
    udp_echo()


if __name__ == "__main__":
    main()
