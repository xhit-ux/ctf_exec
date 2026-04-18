import React, { createContext, useContext, useState, ReactNode } from 'react';
import { ProblemPublishState, initialProblemState } from './types';

interface ProblemPublishContextType {
  state: ProblemPublishState;
  updateState: (updates: Partial<ProblemPublishState>) => void;
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

  const updateState = (updates: Partial<ProblemPublishState>) => {
    setState((prev) => ({ ...prev, ...updates }));
  };

  const setStep = (step: number) => {
    setCurrentStep(step);
  };

  const nextStep = () => {
    setCurrentStep((prev) => Math.min(prev + 1, 5));
  };

  const prevStep = () => {
    setCurrentStep((prev) => Math.max(prev - 1, 1));
  };

  return (
    <ProblemPublishContext.Provider
      value={{
        state,
        updateState,
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
