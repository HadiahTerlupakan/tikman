import type { CreateOdpDto } from "@/domain/entities";

/** Where a distribution box hangs from, as the form asks it. */
export type ParentKind = "odc" | "pon";

export interface OdpFormValues {
  name: string;
  code?: string;
  portCount: number;
  address?: string;
  notes?: string;
  parentKind: ParentKind;
  odcId?: string;
  oltId?: string;
  slot?: number;
  portId?: number;
}

export interface Coordinates {
  latitude: number;
  longitude: number;
}

/**
 * Builds the request for a distribution box, carrying exactly one parent.
 *
 * The form offers both ways of hanging a box — under a cabinet, or straight off
 * a PON port — and leaving the unused half in place would send two parents to a
 * server that refuses them. Dropping it here means the operator sees a form
 * error rather than a rejected save.
 */
export function buildOdpDto(
  values: OdpFormValues,
  coordinates: Coordinates | undefined,
): CreateOdpDto {
  const dto: CreateOdpDto = {
    name: values.name.trim(),
    code: values.code?.trim() || undefined,
    portCount: values.portCount,
    address: values.address?.trim() || undefined,
    notes: values.notes?.trim() || undefined,
    latitude: coordinates?.latitude,
    longitude: coordinates?.longitude,
  };

  if (values.parentKind === "odc") {
    dto.odcId = values.odcId;
    return dto;
  }

  dto.oltId = values.oltId;
  dto.slot = values.slot;
  dto.portId = values.portId;
  return dto;
}

/**
 * What is still missing before a box can be saved, in the operator's words.
 *
 * Returns null when nothing is. The server and the database refuse an
 * incomplete parent too; this only says so before the round trip.
 */
export function odpFormProblem(
  values: OdpFormValues,
  coordinates: Coordinates | undefined,
): string | null {
  if (!coordinates) {
    return "Klik di peta untuk menentukan lokasinya";
  }
  if (values.parentKind === "odc" && !values.odcId) {
    return "Pilih ODC induknya";
  }
  if (values.parentKind === "pon") {
    if (!values.oltId) {
      return "Pilih OLT induknya";
    }
    if (values.slot === undefined || values.portId === undefined) {
      return "Isi slot dan port PON-nya";
    }
  }
  return null;
}
