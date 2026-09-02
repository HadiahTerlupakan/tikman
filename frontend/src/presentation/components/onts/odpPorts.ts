/**
 * The ports still free on a distribution box.
 *
 * Offering a taken port and letting the server refuse it wastes the trip and
 * tells a technician nothing about where the subscriber can actually go. The
 * current subscriber's own port stays on the list, because re-patching to the
 * port already held should not read as a conflict.
 */
export function freePorts(
  portCount: number,
  takenPorts: number[],
  keepPort?: number,
): number[] {
  const taken = new Set(takenPorts.filter((port) => port !== keepPort));
  const ports: number[] = [];
  for (let port = 1; port <= portCount; port++) {
    if (!taken.has(port)) {
      ports.push(port);
    }
  }
  return ports;
}
