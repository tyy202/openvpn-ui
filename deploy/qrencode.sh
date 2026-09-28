#!/bin/sh
exec /usr/bin/qrencode -t PNG -o - "$1"
