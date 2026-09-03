export interface IPushRepository {
  subscribe(fid: string): Promise<void>;
  unsubscribe(fid: string): Promise<void>;
}
