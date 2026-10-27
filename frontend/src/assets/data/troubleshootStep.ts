import type { main } from "../../../wailsjs/go/models";

export function getWindowsSteps(info: main.TroubleshootInfo) {
  return [
    ...defaultSteps(info),
    {
      title: "Windows Firewall Rule",
      desc: "If Windows Defender Firewall is blocking other devices on your local network, open *PowerShell as Administrator* and run the following command.\n\nIf the command does not work, you can create the same rule from *Windows Defender Firewall → Advanced settings → Inbound Rules → New Rule/Edit Rule (if one already exists)*.",
      codes: [`New-NetFirewallRule -DisplayName "Obox App" -Direction Inbound \`\n  -Program "${info.execPath}" \`\n  -Action Allow -Profile Private`,],
    },
    ...networkSteps(info),
  ];
}

export function getMacSteps(info: main.TroubleshootInfo) {
  return [
    ...defaultSteps(info),
    {
      title: "macOS Application Firewall",
      desc: "macOS may block incoming network access to Obox App through its built-in Application Firewall.\n\nYou can allow it in *System Settings → Privacy & Security → Firewall*, or run this in Terminal:",
      codes: [`sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp "/Applications/Obox App.app"`,],
    },
    ...networkSteps(info),
  ];
}

function linuxFirewalldSteps(info: main.TroubleshootInfo) {
  const { port, firewallZone } = info;
  return [
    ...defaultSteps(info),
    {
      title: "Allow Port in Firewalld",
      desc: `*firewalld* is active. Run the following commands in your terminal to allow port *${port}* in the *${firewallZone}* zone:`,
      codes: [`sudo firewall-cmd --permanent --zone=${firewallZone} --add-port=${port}/tcp\nsudo firewall-cmd --reload`,],
    },
    ...networkSteps(info),
  ];
}

function linuxUfwSteps(info: main.TroubleshootInfo) {
  const { port, subnet } = info;

  return [
    ...defaultSteps(info),
    {
      title: "Allow Port in UFW",
      desc: `*ufw* is active on your system. Run this command in your terminal to allow printing from your local network (*${subnet}*):`,
      codes: [`sudo ufw allow from ${subnet} to any port ${port} proto tcp`,],
    },
    ...networkSteps(info),
  ];
}

function linuxNftablesSteps(info: main.TroubleshootInfo) {
  const { port, subnet } = info;
  return [
    ...defaultSteps(info),
    {
      title: "Allow Port in nftables",
      desc: `*nftables* is active. Add a rule to allow incoming TCP traffic on port *${info.port}* from your local network (*${info.subnet}*).\n\n⚠️ This rule is not persistent across reboots. Save it to your nftables config (e.g. \`/etc/nftables.conf\`) to make it permanent.`,
      codes: [`sudo nft add rule inet filter input ip saddr ${subnet} tcp dport ${port} accept`,],
    },
    ...networkSteps(info),
  ];
}

function linuxNoFirewallSteps(info: main.TroubleshootInfo) {
  return [
    ...defaultSteps(info),
    {
      title: "Linux Firewall Rule",
      desc: `Install any firewall package like ufw, firewalld or nftables and allow port ${info.port} for incoming connections. Then open this dialog again.`,
    },
    ...networkSteps(info),
  ];
}

function defaultSteps({ localIp, port }: main.TroubleshootInfo) {
  return [
    {
      title: "Check Obox App Server Accessibility",
      desc: `Check if this Obox App server is accessible from your POS device by opening *http://${localIp}:${port}* in its browser.\n\nIf a page is displayed, the server is accessible. If not, click Next.`,
    },
  ];
}

export function staticIpAdvice(localIp: string) {
  return `To ensure your printer connection never breaks, *reserve a fixed / static IP* for this computer (*${localIp}*) in your router's DHCP settings.\nAfter changing it, restart the app before configuring your POS device.`;
}

function networkSteps({ localIp, port }: main.TroubleshootInfo) {
  return [
    {
      title: "Check Network & Wi-Fi Connection",
      desc: `• *Same Local Wi-Fi:* Ensure your POS device is connected to the same Wi-Fi network (not a Guest network or cellular data).\n\n• *Router Client Isolation:* Check if your Wi-Fi router has "Client Isolation" or "AP Isolation" enabled. This prevents devices from communicating with each other.`,
    },
    {
      title: "Set a Fixed / Static IP",
      desc: staticIpAdvice(localIp),
    },
  ];
}

export function getLinuxSteps(info: main.TroubleshootInfo) {
  if (info.activeFirewall === "firewalld") {
    return linuxFirewalldSteps(info);
  }
  if (info.activeFirewall === "ufw") {
    return linuxUfwSteps(info);
  }
  if (info.activeFirewall === "nftables") {
    return linuxNftablesSteps(info);
  }
  return linuxNoFirewallSteps(info);
}
