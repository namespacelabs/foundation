# This image is provided by the Cypress team and contains the Cypress core and all the supported browsers.
FROM cypress/included:10.11.0

# Install git, supervisor, VNC, & X11 packages
RUN set -ex; \
    apt-get update; \
    apt-get install -y \
      bash \
      fluxbox \
      git \
      net-tools \
      novnc \
      supervisor \
      x11vnc \
      xvfb

# Setup demo environment variables
ENV HOME=/root \
    DEBIAN_FRONTEND=noninteractive \
    LANG=en_US.UTF-8 \
    LANGUAGE=en_US.UTF-8 \
    LC_ALL=C.UTF-8 \
    DISPLAY=:0.0 \
    DISPLAY_WIDTH=1280 \
    DISPLAY_HEIGHT=720

# Copy our test sources and the liveserver run config
COPY liveserver /liveserver

# Making "/" serving path work.
RUN cp /usr/share/novnc/vnc.html /usr/share/novnc/index.html

WORKDIR /app
COPY cypress cypress
COPY cypress.config.js .

