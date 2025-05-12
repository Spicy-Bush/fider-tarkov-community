import React, { createContext, useContext, useState, useEffect, ReactNode } from "react";
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
}

const defaultContext: UserStandingContextType = {
  isLoading: true,
  isMuted: false,
  muteReason: "",
  warnings: [],
  mutes: [],
  refetch: async () => {}
};

const UserStandingContext = createContext<UserStandingContextType>(defaultContext);

export const UserStandingProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const fider = useFider();
  const [isLoading, setIsLoading] = useState(true);
  const [isMuted, setIsMuted] = useState(false);
  const [muteReason, setMuteReason] = useState("");
  const [warnings, setWarnings] = useState<Warning[]>([]);
  const [mutes, setMutes] = useState<Mute[]>([]);

  const fetchUserStanding = async () => {
    if (!fider.session.isAuthenticated) {
      setIsLoading(false);
      return;
    }

    try {
      setIsLoading(true);
      const result = await actions.getUserProfileStanding(fider.session.user.id);
      
      if (result.ok) {
        setWarnings(result.data.warnings);
        setMutes(result.data.mutes);
        
        const now = new Date();
        const activeMute = result.data.mutes.find(mute => 
          !mute.expiresAt || new Date(mute.expiresAt) > now
        );
        
        if (activeMute) {
          setIsMuted(true);
          setMuteReason(activeMute.reason);
        } else {
          setIsMuted(false);
          setMuteReason("");
        }
      }
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchUserStanding();
  }, [fider.session.isAuthenticated]);

  return (
    <UserStandingContext.Provider 
      value={{ 
        isLoading, 
        isMuted, 
        muteReason, 
        warnings, 
        mutes, 
        refetch: fetchUserStanding 
      }}
    >
      {children}
    </UserStandingContext.Provider>
  );
};

export const useUserStanding = () => useContext(UserStandingContext); 