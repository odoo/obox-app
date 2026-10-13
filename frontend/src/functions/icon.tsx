import type { SVGProps } from "react";

export type IconProps = SVGProps<SVGSVGElement>;

const Base24Svg = ({ className = "w-4 h-4", children, ...props }: IconProps) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={className}
    {...props}
  >
    {children}
  </svg>
);

export const WifiIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M5 12.55a11 11 0 0 1 14.08 0" />
    <path d="M1.42 9a16 16 0 0 1 21.16 0" />
    <path d="M8.53 16.11a6 6 0 0 1 6.95 0" />
    <circle cx="12" cy="20" r="1" fill="currentColor" />
  </Base24Svg>
);

export const WarningIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3" />
    <line x1="12" y1="9" x2="12" y2="13" />
    <line x1="12" y1="17" x2="12.01" y2="17" />
  </Base24Svg>
);

export const BellIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path
      d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"
      fill="currentColor"
    />
    <path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" fill="currentColor" />
    <path d="M4 2C2.8 3.7 2 5.7 2 8" />
    <path d="M22 8c0-2.3-.8-4.3-2-6" />
  </Base24Svg>
);

export const SearchIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <circle cx="11" cy="11" r="8" />
    <line x1="21" y1="21" x2="16.65" y2="16.65" />
  </Base24Svg>
);

export const PrinterIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <polyline points="6 9 6 2 18 2 18 9" />
    <path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2" />
    <rect x="6" y="14" width="12" height="8" />
  </Base24Svg>
);

export const CheckIcon = ({ strokeWidth = 2.5, ...props }: IconProps) => (
  <Base24Svg strokeWidth={strokeWidth} {...props}>
    <polyline points="20 6 9 17 4 12" />
  </Base24Svg>
);

export const CloseIcon = ({ strokeWidth = 2.5, ...props }: IconProps) => (
  <Base24Svg strokeWidth={strokeWidth} {...props}>
    <path d="M6 18L18 6M6 6l12 12" />
  </Base24Svg>
);

export const TrashIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
  </Base24Svg>
);

export const CopyIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
  </Base24Svg>
);

export const LinkIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
  </Base24Svg>
);

export const DisconnectIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
  </Base24Svg>
);

export const SettingsIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
    <path d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
  </Base24Svg>
);

export const BoltIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M13 10V3L4 14h7v7l9-11h-7z" />
  </Base24Svg>
);

export const DownloadIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
  </Base24Svg>
);

export const HelpCircleIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
  </Base24Svg>
);

export const AppIdIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
  </Base24Svg>
);

export const IpIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
  </Base24Svg>
);

export const LanStatusIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M8.288 15.038a5.25 5.25 0 017.424 0M5.106 11.856c3.807-3.808 9.98-3.808 13.788 0M1.924 8.674c5.565-5.565 14.587-5.565 20.152 0M12.53 18.22l-.53.53-.53-.53a.75.75 0 011.06 0z" />
  </Base24Svg>
);

export const CloudStatusIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M2.25 15a4.5 4.5 0 004.5 4.5H18a3.75 3.75 0 00.375-7.481A5.25 5.25 0 008.25 7.5a5.228 5.228 0 00-4.024 1.884A4.5 4.5 0 002.25 15z" />
  </Base24Svg>
);

export const UsbIcon = ({ className = "w-4 h-4", ...props }: IconProps) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 640 640"
    fill="currentColor"
    className={className}
    {...props}
  >
    <path d="M633.5 320C633.5 323.1 631.8 326.1 629 327.5L539.9 381C538.5 381.8 537.1 382.4 535.4 382.4C534 382.4 532.3 382.1 530.9 381.3C528.1 379.6 526.4 376.8 526.4 373.5L526.4 337.9L295.7 337.9C321 377.5 336.2 444.8 365.3 444.8L392 444.8L392 418C392 413 395.9 409.1 400.9 409.1L490 409.1C495 409.1 498.9 413 498.9 418L498.9 507.1C498.9 512.1 495 516 490 516L400.9 516C395.9 516 392 512.1 392 507.1L392 480.4L365.3 480.4C289.9 480.4 284.2 337.9 240.6 337.9L140.3 337.9C132.2 368.5 104.4 391.4 71.3 391.4C32 391.3 0 359.3 0 320C0 280.7 32 248.7 71.3 248.7C104.4 248.7 132.3 271.5 140.3 302.2C179.4 302.2 184.2 311.7 214.9 241.8C255 152.7 273 159.7 323.8 159.7C331.3 138.8 350.8 124.1 374.2 124.1C403.7 124.1 427.7 148 427.7 177.6C427.7 207.2 403.8 231.1 374.2 231.1C350.8 231.1 331.3 216.3 323.8 195.5L294 195.5C264.9 195.5 249.7 262.9 224.4 302.4L526.5 302.4L526.5 266.8C526.5 263.5 528.2 260.7 531 259C533.8 257.3 537.4 257.6 539.9 259.3L629 312.8C631.8 313.9 633.5 316.9 633.5 320z" />
  </svg>
);

export const PrinterTypeIcon = ({
  printer,
  className = "w-4 h-4",
}: {
  printer: { isLAN: boolean };
  className?: string;
}) => {
  if (printer.isLAN) return <WifiIcon className={className} />;
  return <UsbIcon className={className} />;
};

export const MonitorIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <rect x="2" y="3" width="20" height="14" rx="2" strokeWidth="2" />
    <line x1="8" y1="21" x2="16" y2="21" strokeWidth="2" />
    <line x1="12" y1="17" x2="12" y2="21" strokeWidth="2" />
  </Base24Svg>
);

export const RefreshIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
    />
  </Base24Svg>
);

export const PlayCircleIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
    <path strokeLinecap="round" strokeLinejoin="round" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
  </Base24Svg>
);

export const SpinnerIcon = ({ className = "w-4 h-4 animate-spin", ...props }: IconProps) => (
  <Base24Svg className={className} stroke="none" strokeWidth={0} {...props}>
    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
  </Base24Svg>
);

export const CheckSolidIcon = (props: IconProps) => (
  <Base24Svg viewBox="0 0 20 20" fill="currentColor" stroke="none" strokeWidth={0} {...props}>
    <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
  </Base24Svg>
);

export const ExclamationCircleSolidIcon = (props: IconProps) => (
  <Base24Svg viewBox="0 0 20 20" fill="currentColor" stroke="none" strokeWidth={0} {...props}>
    <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
  </Base24Svg>
);

export const CheckCircleSolidIcon = (props: IconProps) => (
  <Base24Svg viewBox="0 0 20 20" fill="currentColor" stroke="none" strokeWidth={0} {...props}>
    <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
  </Base24Svg>
);

export const ExclamationTriangleIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3m0 4h.01M10.29 3.86l-8.1 14a2 2 0 001.73 3h16.16a2 2 0 001.73-3l-8.1-14a2 2 0 00-3.42 0z" />
  </Base24Svg>
);

export const DesktopIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
  </Base24Svg>
);

export const FullscreenIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
  </Base24Svg>
);

export const WindowIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <rect x="3" y="3" width="18" height="18" rx="2" strokeWidth="2" />
    <line x1="3" y1="9" x2="21" y2="9" strokeWidth="2" />
  </Base24Svg>
);

export const FingerprintIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M12 11c0 3.517-1.009 6.799-2.753 9.571m-3.44-2.04l.054-.09A13.916 13.916 0 008 11a4 4 0 118 0c0 1.017-.07 2.019-.203 3m-2.118 6.844A21.88 21.88 0 0015.171 17m3.839 1.132c.645-2.266.99-4.659.99-7.132A8 8 0 008 4.07M3 15.364c.64-1.319 1-2.8 1-4.364 0-1.457.39-2.823 1.07-4" />
  </Base24Svg>
);

export const DeviceMobileIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z" />
  </Base24Svg>
);

export const GlobeAltIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <circle cx="12" cy="12" r="10" />
    <line x1="2" y1="12" x2="22" y2="12" />
    <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
  </Base24Svg>
);

export const ExternalLinkIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
    <polyline points="15 3 21 3 21 9" />
    <line x1="10" y1="14" x2="21" y2="3" />
  </Base24Svg>
);

export const ServerStackIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <ellipse cx="12" cy="5" rx="9" ry="3" />
    <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5" />
    <path d="M3 12c0 1.66 4 3 9 3s9-1.34 9-3" />
  </Base24Svg>
);

