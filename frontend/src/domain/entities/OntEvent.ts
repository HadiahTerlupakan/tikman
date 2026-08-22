export interface ONTEvent {
  id: number;
  ontId: string;
  eventType: "online" | "offline";
  eventTime: string;
  reason?: string;
  durationSeconds?: number;
  createdAt: string;
}

export interface AvailabilityStats {
  ontId: string;
  startTime: string;
  endTime: string;
  totalSeconds: number;
  onlineSeconds: number;
  offlineSeconds: number;
  availabilityPercent: number;
  totalEvents: number;
  onlineEvents: number;
  offlineEvents: number;
  mtbf: number;
  mttr: number;
}

export interface ONTEventsResponse {
  events: ONTEvent[];
  total: number;
  limit: number;
  offset: number;
}
