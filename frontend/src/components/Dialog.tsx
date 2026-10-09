import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useMountTransition } from "../hooks/useMountTransition";
import CloseButton from "./CloseButton";

export type ActionType = "primary" | "secondary" | "danger"

export interface DialogAction {
  name: string;
  label: string;
  onClick?: (helpers: { close: () => void }) => void | boolean | Promise<boolean | void>;
  disabled?: boolean;
  variant?: ActionType;
  className?: string;
}

interface DialogProps {
  title: string;
  openButton?: React.ReactNode;
  children: React.ReactNode;
  actions?: DialogAction[];
  onClose?: () => void;
  onOpen?: () => void;
  maxWidth?: string;
  showTitleDivider?: boolean;
  /** Bump this value (e.g. a counter) to open the dialog programmatically, without an openButton. */
  openSignal?: number;
}

export default function Dialog({
  title,
  children,
  openButton,
  actions = [],
  onClose,
  onOpen,
  showTitleDivider = false,
  maxWidth = "max-w-sm",
  openSignal,
}: DialogProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const { mounted } = useMountTransition(isOpen);
  const previousOpenSignal = useRef(openSignal);

  const close = () => {
    setIsOpen(false);
    onClose?.();
  };

  const open = () => {
    setIsOpen(true);
    onOpen?.();
  };

  useEffect(() => {
    if (openSignal === undefined || openSignal === previousOpenSignal.current) {
      return;
    }

    previousOpenSignal.current = openSignal;
    open();
  }, [openSignal]);

  const isExecuting = useRef(false);

  const handleAction = async (action: DialogAction) => {
    if (action.disabled || isExecuting.current) {
      return;
    }

    if (!action.onClick) {
      close();
      return;
    }

    isExecuting.current = true;
    setLoadingAction(action.name);
    try {
      const result = await action.onClick({ close });
      if (result !== false) {
        close();
      }
    } finally {
      isExecuting.current = false;
      setLoadingAction(null);
    }
  };

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        close();
        return;
      }

      if (event.key !== "Enter" || event.repeat) {
        return;
      }

      if ((event.target as HTMLElement | null)?.closest("button, a")) {
        return;
      }

      const primaryAction = actions.find((action) => !action.disabled && action.variant !== "secondary");
      if (primaryAction && !isExecuting.current) {
        handleAction(primaryAction);
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen, actions, onClose]);

  return (
    <>
      {openButton && <div onClick={() => open()}>{openButton}</div>}
      {mounted &&
        createPortal(
          <div
            className={`fixed inset-0 z-50 flex items-end sm:items-center justify-center p-4 transition ${isOpen
              ? "opacity-100 duration-200 ease-out"
              : "opacity-0 duration-150 ease-in"
              }`}
          >
            <div
              className="absolute inset-0 bg-black/75"
              onClick={() => close()}
            />

            <div className={`relative bg-white rounded-2xl w-full ${maxWidth} max-h-[calc(100vh-2rem)] shadow-xl overflow-y-auto overflow-x-hidden p-6 transition-all duration-200 ease-out transform ${isOpen ? "opacity-100 scale-100 translate-y-0" : "opacity-0 scale-95 translate-y-2"} ${showTitleDivider ? "pt-4" : ""}`}>
              <div className={`flex items-center justify-between  ${showTitleDivider ? "pb-3 mb-4 border-b border-gray-200" : "mb-5"}`}>
                <div className="text-lg font-medium">{title}</div>
                <CloseButton onClick={() => close()} />
              </div>

              <div>{children}</div>

              {actions.length > 0 && (
                <div className="flex items-center gap-2 pt-4">
                  {actions.map((action) => {
                    const isActionLoading = loadingAction === action.name;
                    const defaultVariantClass =
                      action.variant === "secondary"
                        ? "border border-gray-300 text-gray-700 bg-white hover:bg-gray-50"
                        : action.variant === "danger"
                          ? "border border-transparent bg-danger text-white hover:opacity-90"
                          : "border border-transparent bg-odoo text-white hover:bg-odoo-dark";

                    return (
                      <button
                        key={action.name}
                        type="button"
                        disabled={Boolean(action.disabled) || isActionLoading || Boolean(loadingAction)}
                        className={action.className ?? `flex-1 rounded-lg px-4 py-2 cursor-pointer text-sm font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${defaultVariantClass}`}
                        onClick={() => handleAction(action)}
                      >
                        {isActionLoading ? "Loading..." : action.label}
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div>,
          document.body,
        )}
    </>
  );
}
