/** PonSubscriber is one customer a technician would look at first. */
export interface PonSubscriber {
  ontId: string;
  label: string;
  name: string;
  trapCount: number;
  downMinutes: number;
}

/** PonNode is one PON port that broke at least one of the two rules. */
export interface PonNode {
  port: number;
  ontCount: number;
  trapPerOnt: number;
  outageShare: number;
  worst: PonSubscriber[];
}

/** CardNode is a line card, present only when one of its ports is in trouble. */
export interface CardNode {
  slot: number;
  ponCount: number;
  pons: PonNode[];
}

/**
 * PonHealth is the pruned tree: only branches in trouble, plus the thresholds
 * that pruned it, so the screen can show the rule instead of applying it
 * invisibly.
 */
export interface PonHealth {
  oltId: string;
  oltName: string;
  medianTrapPerOnt: number;
  trapThreshold: number;
  outageThreshold: number;
  cards: CardNode[];
}
