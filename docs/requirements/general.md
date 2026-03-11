Okay here is the thing:
- you are able to verify everything directly because you also have ssh to the device; so you can see if the device has internet or not

Here are the problems I notice:
- wifi
  - the disconnect button does not work
  - what is the toogle for in the list of configured wifis?
  - I do not like that we directly scan for network when visiting the page. better make it as a button and open a popup where we see the results and can then connect.
  - what is the wifi mode down there for? nothing selected and what will the usage should like?
- network
  - connected client (connected since show invalid date)
- VPN
  - hm dont show things when they are not installed. grey it out and mentioned that it need to be installed with link to services page maybe
- services
  - does installing really do something? can we somehow see some results or something (maybe also popop with an option to see the log output)
  - We probably need configuration options or more details per service then for example for adguard I want to see if we have configured it to be used as default DNS, maybe an option for playing together with VPNs, the IP of adguards UI
- system
  - reboot does not nothing probably the other buttons also not?
- whats the Clients things at the bottom supposed to do?

Missing features:
  - how do we behave with multiple radios we have on the devices? not sure what network wise is best
  - deleting existing wifis
  - set static IPs for connected clients
  - where to set the DHCP range?
  - what about usb tethering?
  - what about fallback of connections which we can check with pings or something (this is advanced and probably not needed now)
  - logs viewer
  - did not test a WAN connection but i hope it does work? how does it play together with WWAN?
  - i think we need more hover information for different things (example wifi icons in the networks to show more details)
  - also where can we configure the own WIFI we span open for the clients. Then also the name, password, should we split 5ghz and 2,4ghz (maybe also ensure what we really have)
  - maybe some things also need to be done by script or something to initially discover what setup we have and store it somewhere? or is the go service fast enough to get all required information during startup? probably
  - what about NTP and stuff? (probably also more advanced)
  - what about timezone. I remember in glinet this will also be dynamically be shown if the configured timezone mismatches the browser or something.
  - What about if the user wants the thing to act as repeater or something?
  - we need to ensure during application start that the wifi is opened to be able to connect to the device
  - when we install adguard; after install the application should do the initial configuration of adguard like assigning the correct port and interfaces; handle the DNS port correctl (not sure what is best move the existing DNS to another port or connect the exsting with adguard)
  - Backup / restore for configurations

Needs validation (ignore for now):
- I think we need to add this also to our startup script (install script / cleanup script): https://forum.openwrt.org/t/gl-inet-ax1800-new-router-openwrt-support/105163/794
