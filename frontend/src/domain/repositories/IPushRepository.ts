export interface IPushRepository {
  subscribe(fcmToken: string): Promise<void>;
  unsubscribe(fcmToken: string): Promise<void>;
}
