/**
 * Severity bands shared by the topology's fills and the ranked table's trap
 * colour.
 *
 * One definition because both read the same normalised score: a port drawn red
 * on the diagram beside a row left plain in the table would be the screen
 * contradicting itself.
 */
export const SEVERITY_HIGH = 0.66;
export const SEVERITY_MEDIUM = 0.33;
