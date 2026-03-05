include $(TOPDIR)/rules.mk

PKG_NAME:=openwrt-travel-gui
PKG_VERSION:=0.1.0
PKG_RELEASE:=1

PKG_MAINTAINER:=openwrt-travel-gui contributors
PKG_LICENSE:=MIT

include $(INCLUDE_DIR)/package.mk

define Package/openwrt-travel-gui
  SECTION:=luci
  CATEGORY:=LuCI
  TITLE:=Modern web UI for OpenWRT travel routers
  DEPENDS:=
  PKGARCH:=all
endef

define Package/openwrt-travel-gui/description
  A modern, mobile-first web UI for GL.iNet travel routers.
  Provides dashboard, WiFi management, captive portal support,
  VPN configuration, and system settings.
endef

define Package/openwrt-travel-gui/install
	$(INSTALL_DIR) $(1)/usr/bin
	$(INSTALL_BIN) ./files/usr/bin/openwrt-travel-gui $(1)/usr/bin/

	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/etc/init.d/openwrt-travel-gui $(1)/etc/init.d/

	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/etc/config/openwrt-travel-gui $(1)/etc/config/

	$(INSTALL_DIR) $(1)/etc/uci-defaults
	$(INSTALL_BIN) ./files/etc/uci-defaults/99-travel-gui-ports $(1)/etc/uci-defaults/

	$(INSTALL_DIR) $(1)/www/openwrt-travel-gui
	$(CP) ./files/www/openwrt-travel-gui/* $(1)/www/openwrt-travel-gui/
endef

define Package/openwrt-travel-gui/conffiles
/etc/config/openwrt-travel-gui
endef

$(eval $(call BuildPackage,openwrt-travel-gui))
