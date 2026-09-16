# БАЗА
FROM alpine:3.19 AS base

RUN apk add --no-cache curl

# TARGET: cmd-only — только CMD
FROM base AS cmd-only
CMD ["echo", "hello from CMD"]

# TARGET: entrypoint-only — только ENTRYPOINT (обёртка над curl)
FROM base AS entrypoint-only
ENTRYPOINT["curl", "-s"]

# TARGET: both — ENTRYPOINT + CMD
FROM base AS both
ENTRYPOINT["curl", "-s"]
CMD["https://httpbin.org/get"]

# TARGET: shell-form — НЕПРАВИЛЬНО (shell-форма)
FROM base AS shell-form

RUN printf '#!/bin/sh\n\
trap "echo CAUGHT SIGTERM; exit 0" TERM\n\
echo "script started, PID=$$"\n\
while true; do sleep 1; done\n' > /entry.sh \
    && chmod +x /entry.sh

ENTRYPOINT /entry.sh

# TARGET: exec-form — ПРАВИЛЬНО (exec-форма)
FROM base AS exec-form

RUN printf '#!/bin/sh\n\
trap "echo CAUGHT SIGTERM; exit 0" TERM\n\
echo "script started, PID=$$"\n\
while true; do sleep 1; done\n' > /entry.sh \
    && chmod +x /entry.sh

ENTRYPOINT ["/entry.sh"]

# TARGET: multi-role — один образ, разные роли через CMD
FROM base AS multi-role

RUN printf '#!/bin/sh\n\
case "$1" in\n\
  server)  echo "SERVER started, PID=$$"; exec sleep 3600 ;;\n\
  worker)  echo "WORKER started, PID=$$"; exec sleep 3600 ;;\n\
  migrate) echo "MIGRATE done"; exit 0 ;;\n\
  *)       echo "usage: $0 {server|worker|migrate}"; exit 1 ;;\n\
esac\n' > /entrypoint.sh && chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["server"]

# TARGET: cmd-override — CMD легко переопределить
FROM base AS cmd-override

RUN printf '#!/bin/sh\necho "Running: $@"\n' > /run.sh && chmod +x /run.sh

ENTRYPOINT ["/run.sh"]
CMD ["default", "arguments"]