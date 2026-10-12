import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import PINModal from "../components/PINModal";
import { getPINSessionToken } from "../api/authState";
import { AppContext } from "./AppContext";

/**
 * PINContext — provides a programmatic `showPINDialog()` that returns a
 * Promise<boolean>, and displays an immediate auth overlay upon refresh / load
 * in remote browser context so users are authenticated right away.
 */

type PINContextType = {
  /** Opens the PIN dialog and resolves to true on success, false on dismiss. */
  showPINDialog: () => Promise<boolean>;
  isAuthenticated: boolean;
};

export const PINContext = createContext<PINContextType>({
  showPINDialog: async () => false,
  isAuthenticated: false,
});

export function usePINContext() {
  return useContext(PINContext);
}

interface PINContextWrapperProps {
  children: React.ReactNode;
}

export function PINContextWrapper({ children }: PINContextWrapperProps) {
  const { data: { isWails } } = useContext(AppContext);
  const [visible, setVisible] = useState(false);
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(() => {
    return isWails || Boolean(getPINSessionToken());
  });

  // Store the resolve function of the current in-flight promise.
  const resolverRef = useRef<((ok: boolean) => void) | null>(null);

  // Upon initial load or refresh in remote webview: immediately prompt for PIN
  useEffect(() => {
    if (!isWails && !getPINSessionToken()) {
      setVisible(true);
    }
  }, [isWails]);

  const showPINDialog = useCallback((): Promise<boolean> => {
    return new Promise<boolean>((resolve) => {
      resolverRef.current = resolve;
      setVisible(true);
    });
  }, []);

  const handleSuccess = useCallback(() => {
    setVisible(false);
    setIsAuthenticated(true);
    resolverRef.current?.(true);
    resolverRef.current = null;
  }, []);

  const handleDismiss = useCallback(() => {
    setVisible(false);
    resolverRef.current?.(false);
    resolverRef.current = null;
  }, []);

  return (
    <PINContext.Provider value={{ showPINDialog, isAuthenticated }}>
      {children}
      {visible && (
        <PINModal
          title="Admin Authentication"
          subtitle="Enter 4-digit PIN to unlock administrative features"
          onSuccess={handleSuccess}
          onDismiss={handleDismiss}
        />
      )}
    </PINContext.Provider>
  );
}
