import { createContext, useCallback, useEffect, useState } from "react";
import { obox } from "../../wailsjs/go/models";
import {
  ConfirmDisconnectOdoo,
  CheckOdooStatus,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";

export type OdooContextType = {
  data: {
    status:  obox.ConnectionStatus | null;
    isConnected: boolean;
  };
  actions: {
    refreshStatus: () => Promise<void>;
    disconnectOdoo: () => Promise<boolean>;
  };
};

export const OdooContext = createContext({} as OdooContextType);

interface OdooContextWrapperProps {
  children: React.ReactNode;
}

export const OdooContextWrapper = ({ children }: OdooContextWrapperProps) => {
  const [status, setStatus] = useState< obox.ConnectionStatus | null>(null);

  const refreshStatus = useCallback(async () => {
    try {
      const odooStatus = await CheckOdooStatus();
      setStatus(odooStatus);
    } catch (error) {
      console.error("Failed to fetch Odoo status:", error);
    }
  }, []);

  const disconnectOdoo = useCallback(async () => {
    try {
      const confirmed = await ConfirmDisconnectOdoo();
      return confirmed;
    } catch (error) {
      console.error("Failed to disconnect Odoo:", error);
      return false;
    }
  }, []);

  useEffect(() => {    
    // Listen to real-time status updates pushed from backend
    const unsubscribe = EventsOn("odoo:status_changed", (newStatus:  obox.ConnectionStatus) => {
      setStatus(newStatus);
    });

    // Initial fetch on mount
    refreshStatus();

    return () => {
      if (unsubscribe) {
        unsubscribe();
      }
    };
  }, [refreshStatus]);

  const isConnected = Boolean(status?.dbUrl);

  const data = { status, isConnected };
  const actions = { refreshStatus, disconnectOdoo };

  return (
    <OdooContext.Provider value={{ data, actions }}>
      {children}
    </OdooContext.Provider>
  );
};

