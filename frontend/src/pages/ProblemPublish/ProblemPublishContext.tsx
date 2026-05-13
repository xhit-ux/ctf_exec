import React, { createContext, useCallback, useContext, useState, ReactNode } from 'react';
import { ProblemPublishState, initialProblemState } from './types';

interface ProblemPublishContextType {
  state: ProblemPublishState;
  updateState: (updates: Partial<ProblemPublishState>) => void;
  resetState: () => void;
  currentStep: number;
  setStep: (step: number) => void;
  nextStep: () => void;
  prevStep: () => void;
  isSubmitting: boolean;
  setSubmitting: (submitting: boolean) => void;
}

const ProblemPublishContext = createContext<ProblemPublishContextType | undefined>(undefined);

export const ProblemPublishProvider = ({ children }: { children: ReactNode }) => {
  const [state, setState] = useState<ProblemPublishState>(initialProblemState);
  const [currentStep, setCurrentStep] = useState(1);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const updateState = useCallback((updates: Partial<ProblemPublishState>) => {
    setState((prev) => ({ ...prev, ...updates }));
  }, []);

  const resetState = useCallback(() => {
    setState({ ...initialProblemState });
  }, []);

  const setStep = useCallback((step: number) => {
    setCurrentStep(step);
  }, []);

  const nextStep = useCallback(() => {
    setCurrentStep((prev) => Math.min(prev + 1, 5));
  }, []);

  const prevStep = useCallback(() => {
    setCurrentStep((prev) => Math.max(prev - 1, 1));
  }, []);

  return (
    <ProblemPublishContext.Provider
      value={{
        state,
        updateState,
        resetState,
        currentStep,
        setStep,
        nextStep,
        prevStep,
        isSubmitting,
        setSubmitting: setIsSubmitting,
      }}
    >
      {children}
    </ProblemPublishContext.Provider>
  );
};

export const useProblemPublish = () => {
  const context = useContext(ProblemPublishContext);
  if (context === undefined) {
    throw new Error('useProblemPublish must be used within a ProblemPublishProvider');
  }
  return context;
};
