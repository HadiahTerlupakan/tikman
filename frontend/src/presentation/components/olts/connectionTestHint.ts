/**
 * connectionTestHint reads a failed connection test in the light of the probes
 * that passed before it.
 *
 * SNMPTest reports "wrong port/community or device unreachable" because that is
 * everything a lone UDP timeout can mean. Once ping and a login have answered,
 * unreachable is off the table, and leaving it in the message sends an operator
 * to check routing that was just proven clear. Returns null whenever the earlier
 * probes settle nothing.
 */
export function connectionTestHint(
  failedTest?: string,
  passedTests: string[] = [],
): string | null {
  if (failedTest !== "SNMP" || !passedTests.includes("Ping")) {
    return null;
  }
  return (
    `${passedTests.join(" and ")} already reached this device, so it is not ` +
    `unreachable — its SNMP agent is not answering. Check snmp-server ` +
    `community on the chassis, then the community and port above.`
  );
}
