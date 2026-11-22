FROM ghcr.io/jlesage/firefox@sha256:9b6f819004868a74a14fb1d82f4b2655a316cc99cf58c60bf41cbdb84bf8a08f

RUN apk add --no-cache chromium font-noto-cjk font-noto-emoji xdotool xprop

RUN sed -i "/window.addEventListener('orientationchange', function () {/,/^        });/d" /opt/noVNC/app/ui.js

RUN printf '%s\n' \
    '#!/bin/sh' \
    '(window=$(timeout 20s xdotool search --sync --onlyvisible --limit 1 --class "^Chromium$") && xprop -id "$window" -remove WM_NORMAL_HINTS) &' \
    'exec /usr/bin/chromium --disable-setuid-sandbox --no-first-run --no-default-browser-check --disable-background-networking --disable-component-update --user-data-dir=/config/chromium --remote-debugging-address=127.0.0.1 --remote-debugging-port=9222 --kiosk --lang=zh-CN https://x.com/i/flow/login' \
    > /startapp.sh
