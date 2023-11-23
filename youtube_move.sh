#!/bin/bash

output_dir="./downloads"

mkdir -p "$output_dir"

source_file="$1"
destination_file="$2"
uploader="$3"
destination_dir_with_uploader="$output_dir/$uploader"

mkdir -p "$destination_dir_with_uploader"
mv "$source_file" "$destination_dir_with_uploader/$destination_file"
echo "Moved: $source_file to $destination_dir_with_uploader"