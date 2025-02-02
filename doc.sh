#! /usr/bin/env sh
ls $1/*.go || exit 0
godocdown $1 -output doc.md