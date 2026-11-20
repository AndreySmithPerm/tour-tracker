#!/bin/sh
# Replace environment variables in nginx config
envsubst '${DOMAIN} ${BOT_URL} ${BOT_HANDLE}' < /etc/nginx/nginx.conf > /etc/nginx/conf.d/default.conf
exec nginx -g 'daemon off;'
