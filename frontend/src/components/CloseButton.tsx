import { CloseIcon } from "../functions/icon";

interface CloseButtonProps {
  onClick: () => void;
}

export default function CloseButton({ onClick }: CloseButtonProps) {
  return (
    <button
      className="w-7 h-7 rounded-lg bg-stone-100 hover:bg-stone-200 flex items-center justify-center text-stone-400 transition-colors cursor-pointer"
      onClick={onClick}
    >
      <CloseIcon className="w-3.5 h-3.5" />
    </button>
  );
}
