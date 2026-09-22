# Vendored zapret nfqws

This directory vendors the Linux `nfqws` packet engine from
[`bol-van/zapret`](https://github.com/bol-van/zapret), release `v72.13`, commit
`87e058624c72863db53bdaf7fb6f16576dddb6ab`.

Included are the Linux `nfqws` C and header sources from upstream `nfq/`, plus
its required `nfq/crypto/` sources. Windows service code, WinDivert binaries,
BSD build files, zapret shell scripts, and unrelated utilities are excluded.
`Makefile` is local build glue; the packet engine sources are upstream files.

The upstream project is MIT licensed. See `THIRD_PARTY_LICENSES/zapret.txt`.
