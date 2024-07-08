#!/bin/bash
INPUT_DIR="./testdata/example_1"
OUTPUT_DIR="./tmp/output"

INIT_FILE="$INPUT_DIR/init-0.mp4"
CHUNK_FILE="$INPUT_DIR/chunk-0-00001.m4s"
TMP_FILE="$OUTPUT_DIR/combined-1.mp4"

cat "$INIT_FILE" "$CHUNK_FILE" > "$TMP_FILE"

ffmpeg -y -i "$TMP_FILE" -vf "drawtext=text='Hello world!':fontfile=/path/to/arial.ttf:fontcolor=white:fontsize=24:box=1:boxcolor=black@0.5:boxborderw=5:x=10:y=10" -c:a copy -f mp4 -movflags +empty_moov+default_base_moof "$OUTPUT_DIR/combined-2.mp4"

rm "$TMP_FILE"
