#!/bin/sh
set -e

# Replace the build-time placeholder with the runtime VITE_API_BASE_URL value.
# This allows a single image to be used across environments.
if [ -z "$VITE_API_BASE_URL" ]; then
  echo "ERROR: VITE_API_BASE_URL is not set." >&2
  exit 1
fi

find /usr/share/nginx/html -name "*.js" | while read -r file; do
  sed -i "s|__WIMP_API_URL__|${VITE_API_BASE_URL}|g" "$file"
  sed -i "s|__WIMP_PROMETHEUS_URL__|${VITE_PROMETHEUS_URL}|g" "$file"
done

exec "$@"
