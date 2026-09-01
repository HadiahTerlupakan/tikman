/** durasi renders minutes the way an operator says them out loud. */
export function durasi(minutes: number): string {
  if (minutes < 60) return `${minutes} mnt`;
  const hours = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `${hours} jam` : `${hours} jam ${rest} mnt`;
}
