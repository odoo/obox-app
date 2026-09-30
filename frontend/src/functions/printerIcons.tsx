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

export const BluetoothIcon = (props: IconProps) => (
  <Base24Svg {...props}>
    <polyline points="6.5 6.5 17.5 17.5 12 23 12 1 17.5 6.5 6.5 17.5" />
  </Base24Svg>
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
  printer: { connectionType?: string };
  className?: string;
}) => {
  if (printer.connectionType === "bluetooth") return <BluetoothIcon className={className} />;
  if (printer.connectionType === "lan") return <WifiIcon className={className} />;
  return <UsbIcon className={className} />;
};
