import { cloneElement, isValidElement, useEffect, useRef, useState } from "react";
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
  showTitleDivider?: boolean;
  /** Bump this value (e.g. a counter) to open the dialog programmatically, without an openButton. */
  openSignal?: number;
  /** Bump this value to close the dialog programmatically. */
  closeSignal?: number;
  maxWidth?: string;
}

function useSignal(signal: number | undefined, callback: () => void) {
  const prevSignal = useRef(signal);
  const callbackRef = useRef(callback);
  callbackRef.current = callback;

  useEffect(() => {
    if (signal === undefined || signal === prevSignal.current) {
      return;
    }
    prevSignal.current = signal;
    callbackRef.current();
  }, [signal]);
}

export default function Dialog({
  title,
  children,
  openButton,
  actions = [],
  onClose,
  onOpen,
  showTitleDivider = false,
  openSignal,
  closeSignal,
  maxWidth = "max-w-sm",
}: DialogProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [loadingAction, setLoadingAction] = useState<string | null>(null);
  const { mounted } = useMountTransition(isOpen);

  const close = () => {
    setIsOpen(false);
    onClose?.();
  };

  const open = () => {
    setIsOpen(true);
    onOpen?.();
  };

  useSignal(openSignal, open);
  useSignal(closeSignal, close);

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
      {isValidElement(openButton) &&
        cloneElement(openButton as React.ReactElement<{ onClick?: (e: React.MouseEvent) => void }>, {
          onClick: (e: React.MouseEvent) => {
            (openButton.props as { onClick?: (e: React.MouseEvent) => void }).onClick?.(e);
            open();
          },
        })}
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

            <div className={`relative bg-white rounded-2xl w-full ${maxWidth} max-h-[calc(100vh-2rem)] shadow-xl overflow-y-auto p-6 ${showTitleDivider ? "pt-4" : ""}`}>
              <div className={`flex items-center justify-between ${showTitleDivider ? "pb-3 mb-4 border-b border-gray-200" : "mb-5"}`}>
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
