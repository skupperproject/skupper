#!/bin/bash

# Source directory
SOURCE_DIR="./config/crd/bases"

# Destination directory
DEST_DIR="./deployments/helm/crds"

# Check if source directory exists
if [ ! -d "$SOURCE_DIR" ]; then
  echo "Source directory $SOURCE_DIR does not exist."
  exit 1
fi

# Check if destination directory exists, create if it does not
if [ ! -d "$DEST_DIR" ]; then
  mkdir -p "$DEST_DIR"
else

  rm -rf "$DEST_DIR"/*
fi


# Copy files from source to destination, excluding files with "sample" in their name
find "$SOURCE_DIR" -type f ! -name '*sample*' -exec cp {} "$DEST_DIR" \;
