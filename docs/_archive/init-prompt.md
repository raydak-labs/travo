I want to start an open source project for an new UI/GUI for OpenWRT. I want to be something like the UI for the GLinet travel routers, so it should be mostly for travel router systems.

- provide me recommendation for the frontend/backend stack
- via the UI things should be done like: installing additional tools and configure them: example tailscale, adguard, wireguard etc
- connection to wifis with differents modes like repeater etc
- WAN connection via ports and configurable
- system stats and more
- the UI should be modern and easy to understand and good structures for configuration
- make sure that the dashboard shows the most important features for a travel router (depending on enabled services) like: connected wifi or lan or mobile network, VPN/Wireguard, tailscale etc and provide quick function for switching wifi or disable things etc
- the whole things should be easy to develop locally so stuff should be able to mock either via mock server or whatever seems fit
- i want to also have support for hotel wifi where we should initially check if we have access to internet and if not open with provided DNS servers to login most likely
- recommend things which i maybe miss
- set this project up as monorepo for required things. if it makes sense and required use docker.
- at the end we should be able to build it and for test easily install on test device. Afterwards recommend how it would be best to deploy on openwrt instances
