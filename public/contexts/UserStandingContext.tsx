import React, { createContext, useContext, useState, useCallback, ReactNode } from "react";
import { actions } from "@fider/services";
import { useFider } from "@fider/hooks";

interface Warning {
  id: number;
  reason: string;
  createdAt: string;
  expiresAt?: string;
}

interface Mute {
  id: number;
  reason: string;
  createdAt: string;
  expiresAt?: string;
}

interface UserStandingContextType {
  isLoading: boolean;
  isMuted: boolean;
  muteReason: string;
  warnings: Warning[];
  mutes: Mute[];
  refetch: () => Promise<void>;
  setStandingData: (warnings: Warning[], mutes: Mute[]) => void;
}

const defaultContext: UserStandingContextType = {
  isLoading: false,
  isMuted: false,
  muteReason: "",
  warnings: [],
  mutes: [],
  refetch: async () => {},
  setStandingData: () => {}
};

const UserStandingContext = createContext<UserStandingContextType>(defaultContext);

export const UserStandingProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const fider = useFider();
  
  const serverIsMuted = fider.session.isAuthenticated ? fider.session.user.isMuted : false;
  
  const [isLoading, setIsLoading] = useState(false);
  const [isMuted, setIsMuted] = useState(serverIsMuted);
  const [muteReason, setMuteReason] = useState("");
  const [warnings, setWarnings] = useState<Warning[]>([]);
  const [mutes, setMutes] = useState<Mute[]>([]);

  const updateMuteStatus = useCallback((mutesList: Mute[]) => {
    const now = new Date();
    const activeMute = mutesList.find(mute => 
      !mute.expiresAt || new Date(mute.expiresAt) > now
    );
    
    if (activeMute) {
      setIsMuted(true);
      setMuteReason(activeMute.reason);
    } else {
      setIsMuted(false);
      setMuteReason("");
    }
  }, []);

  const setStandingData = useCallback((newWarnings: Warning[], newMutes: Mute[]) => {
    setWarnings(newWarnings);
    setMutes(newMutes);
    updateMuteStatus(newMutes);
  }, [updateMuteStatus]);

  const fetchUserStanding = useCallback(async () => {
    if (!fider.session.isAuthenticated) {
      return;
    }

    try {
      setIsLoading(true);
      const result = await actions.getUserProfileStanding(fider.session.user.id);
      
      if (result.ok) {
        setStandingData(result.data.warnings, result.data.mutes);
      }
    } finally {
      setIsLoading(false);
    }
  }, [fider.session.isAuthenticated, fider.session.isAuthenticated ? fider.session.user.id : undefined, setStandingData]);

  return (
    <UserStandingContext.Provider 
      value={{ 
        isLoading, 
        isMuted, 
        muteReason, 
        warnings, 
        mutes, 
        refetch: fetchUserStanding,
        setStandingData
      }}
    >
      {children}
    </UserStandingContext.Provider>
  );
};

export const useUserStanding = () => useContext(UserStandingContext); 