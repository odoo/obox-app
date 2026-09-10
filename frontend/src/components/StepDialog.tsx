import { useEffect, useRef, useState } from "react";
import { useClipboard } from "../hooks/useClipboard";
import { BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import type { Step } from "../types";
import { useStepDialog } from "../hooks/useStepDialog";
import { renderFormattedText } from "../functions/renderFormattedText";
import Dialog, { type DialogAction } from "./Dialog";

type ContentPhase = "shown" | "leaving" | "entering";

const CONTENT_PHASE_CLASS: Record<ContentPhase, string> = {
  leaving: "opacity-0 -translate-x-4 transition duration-200 ease-in",
  entering: "opacity-0 translate-x-4",
  shown: "opacity-100 translate-x-0 transition duration-300 ease-out",
};

interface StepDialogProps {
  steps: Step[];
  openButton: React.ReactNode;
  title?: string;
  isLoading?: boolean;
  onOpen?: () => void;
}

export default function StepDialog({ steps, openButton, title, isLoading, onOpen }: StepDialogProps) {
  const [contentEl, setContentEl] = useState<HTMLDivElement | null>(null);
  const { currentStep, next, back, setCurrentStep } = useStepDialog();
  const [displayStep, setDisplayStep] = useState(0);
  const [phase, setPhase] = useState<ContentPhase>("shown");
  const [contentHeight, setContentHeight] = useState("auto");
  const { copy, isCopied } = useClipboard();
  const [scrollingCodes, setScrollingCodes] = useState<Set<number>>(new Set());
  const codeRefs = useRef<(HTMLPreElement | null)[]>([]);

  useEffect(() => {
    if (currentStep === displayStep) {
      return;
    }

    setPhase("leaving");
    const timeout = setTimeout(() => {
      setDisplayStep(currentStep);
      setPhase("entering");
    }, 200);

    return () => clearTimeout(timeout);
  }, [currentStep, displayStep]);

  useEffect(() => {
    if (!contentEl) {
      // Drop the pinned height so the next open measures from scratch instead
      // of clipping the first step against the last one's height.
      setContentHeight("auto");
      return;
    }

    const observer = new ResizeObserver(() =>
      setContentHeight(`${contentEl.offsetHeight}px`),
    );

    observer.observe(contentEl);
    return () => observer.disconnect();
  }, [contentEl]);

  useEffect(() => {
    if (!contentEl) {
      return;
    }

    // Force scroll only on overflowing code boxes for correct height
    // measurement, then relax back to auto. Reruns per step since blocks
    // remount when navigating back.
    const overflowing = new Set<number>();
    codeRefs.current.forEach((el, index) => {
      if (el && el.scrollWidth > el.clientWidth) {
        overflowing.add(index);
      }
    });
    setScrollingCodes(overflowing);

    const raf = requestAnimationFrame(() =>
      requestAnimationFrame(() => setScrollingCodes(new Set())),
    );

    return () => cancelAnimationFrame(raf);
  }, [contentEl, displayStep]);

  useEffect(() => {
    if (phase !== "entering") {
      return;
    }

    let inner = 0;
    const outer = requestAnimationFrame(() => {
      inner = requestAnimationFrame(() => setPhase("shown"));
    });

    return () => {
      cancelAnimationFrame(outer);
      cancelAnimationFrame(inner);
    };
  }, [phase]);

  const step = steps[displayStep];

  const handleOpen = () => {
    setCurrentStep(0);
    if (onOpen) {
      onOpen();
    }
  };

  const dialogTitle = title || (step ? step.title : "Steps");
  const dialogActions: DialogAction[] = (() => {
    if (isLoading || steps.length === 0) {
      return [];
    }

    const nextAction: DialogAction = { name: "next", label: "Next", variant: "primary", onClick: () => { next(steps.length); return false; } }
    const done: DialogAction = { name: "done", label: "Done", variant: "primary", onClick: () => { setCurrentStep(0); return true; }, };
    const backAction: DialogAction = { name: "back", label: "Back", variant: "secondary", onClick: () => { back(); return false; } }

    if (currentStep === 0) {
      return [nextAction];
    }

    if (currentStep === steps.length - 1) {
      return [backAction, done];
    }

    return [backAction, nextAction];
  })();

  return (
    <Dialog openButton={openButton} title={dialogTitle} onOpen={handleOpen} actions={dialogActions} showTitleDivider>
      {isLoading ? (
        <div className="py-8 flex flex-col items-center justify-center text-center gap-2">
          <div className="w-5 h-5 border-2 border-odoo border-t-transparent rounded-full animate-spin" />
          <span className="text-sm text-gray-600 font-medium">
            Loading guide...
          </span>
        </div>
      ) : steps.length === 0 ? (
        <div className="py-6 text-center text-gray-600 text-sm">
          No steps required. Your setup is ready.
        </div>
      ) : (
        <>
          {steps.length > 1 && (
            <div className="flex items-center justify-between mb-4 text-xs text-gray-600 font-medium">
              <span>
                Step {displayStep + 1} of {steps.length}
              </span>
              <div className="flex items-center gap-1.5">
                {steps.map((_, i) => (
                  <div
                    key={i}
                    className={`h-1.5 rounded-full transition-all duration-300 ${i === displayStep
                      ? "w-6 bg-odoo"
                      : i < displayStep
                        ? "w-2.5 bg-odoo/40"
                        : "w-2.5 bg-stone-200"
                      }`}
                  />
                ))}
              </div>
            </div>
          )}

          <div
            className="overflow-hidden transition-[height] duration-300 ease-in-out"
            style={{ height: contentHeight }}
          >
            <div ref={setContentEl} className={`${CONTENT_PHASE_CLASS[phase]}`}>
              <h3 className="font-semibold text-stone-900 text-base mb-2">
                {renderFormattedText(step.title)}
              </h3>
              <p className="text-gray-600 text-sm whitespace-pre-line leading-relaxed">
                {renderFormattedText(step.desc)}
              </p>

              {step.link && (
                <a
                  href={step.link}
                  target="_blank"
                  rel="noreferrer"
                  onClick={(event) => {
                    event.preventDefault();
                    BrowserOpenURL(step.link!);
                  }}
                  className="inline-flex items-center mt-3 px-3 py-2 rounded-lg border border-gray-300 text-gray-600 hover:bg-gray-50 hover:border-gray-400 text-sm transition font-medium"
                >
                  {step.linkLabel}
                </a>
              )}

              {step.image && (
                <img
                  src={step.image}
                  alt={step.title}
                  className="max-w-full mt-3 rounded-lg border border-gray-200 shadow-xs"
                />
              )}

              {step.codes?.map((code, index) => (
                <div key={index} className="mt-3 relative group">
                  <pre
                    ref={(el) => {
                      codeRefs.current[index] = el;
                    }}
                    className={`bg-slate-800 text-emerald-500 text-xs rounded-lg px-4 py-3 font-mono ${scrollingCodes.has(index) ? "overflow-x-scroll" : "overflow-x-auto"}`}
                  >
                    {code}
                  </pre>
                  <button
                    className="absolute top-2.5 right-2 px-2 py-1 text-xs rounded-md bg-slate-700 text-slate-300 hover:bg-slate-600 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                    onClick={() => copy(code, `code-${index}`, "Command")}
                  >
                    {isCopied(`code-${index}`) ? "✓ Copied" : "Copy"}
                  </button>
                </div>
              ))}
            </div>
          </div>
        </>
      )}
    </Dialog>
  );
}
