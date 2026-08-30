import { useState } from "react";
import {
  APIProvider,
  InfoWindow,
  Map,
  Marker,
} from "@vis.gl/react-google-maps";
import { OltStatus, type Olt, type Site } from "@/domain/entities";

interface SiteMapProps {
  apiKey: string;
  sites: Site[];
  olts: Olt[];
}

// Indonesia, so an installation with no pins yet opens somewhere recognisable
// rather than in the Atlantic at 0,0.
const FALLBACK_CENTER = { lat: -2.5, lng: 118 };
const FALLBACK_ZOOM = 4;
// A single pin should not open at maximum zoom, where there is no context.
const SINGLE_SITE_ZOOM = 14;

/** Sites carrying both coordinates, which are the only ones that can be drawn. */
export function mappedSites(sites: Site[] | undefined): Site[] {
  return (sites ?? []).filter(
    (site) =>
      typeof site.latitude === "number" && typeof site.longitude === "number",
  );
}

/** Sites the map cannot place, which the page must still account for. */
export function unmappedSites(sites: Site[] | undefined): Site[] {
  return (sites ?? []).filter(
    (site) =>
      typeof site.latitude !== "number" || typeof site.longitude !== "number",
  );
}

export function SiteMap({ apiKey, sites, olts }: SiteMapProps) {
  const [selected, setSelected] = useState<Site | null>(null);
  const pins = mappedSites(sites);

  const center =
    pins.length > 0
      ? { lat: pins[0].latitude as number, lng: pins[0].longitude as number }
      : FALLBACK_CENTER;
  const zoom = pins.length > 0 ? SINGLE_SITE_ZOOM : FALLBACK_ZOOM;

  return (
    <APIProvider apiKey={apiKey}>
      <Map
        style={{ width: "100%", height: 520, borderRadius: 8 }}
        defaultCenter={center}
        defaultZoom={zoom}
        gestureHandling="greedy"
        disableDefaultUI={false}
      >
        {pins.map((site) => (
          <Marker
            key={site.id}
            position={{
              lat: site.latitude as number,
              lng: site.longitude as number,
            }}
            title={site.name}
            onClick={() => setSelected(site)}
          />
        ))}

        {selected && (
          <InfoWindow
            position={{
              lat: selected.latitude as number,
              lng: selected.longitude as number,
            }}
            onCloseClick={() => setSelected(null)}
          >
            <SiteSummary site={selected} olts={olts} />
          </InfoWindow>
        )}
      </Map>
    </APIProvider>
  );
}

function SiteSummary({ site, olts }: { site: Site; olts: Olt[] }) {
  const owned = olts.filter((olt) => olt.siteId === site.id);
  const online = owned.filter((olt) => olt.status === OltStatus.ONLINE).length;

  return (
    <div style={{ color: "#18181b", minWidth: 160 }}>
      <div style={{ fontWeight: 600 }}>{site.name}</div>
      <div style={{ fontSize: 12 }}>{site.location || "no address"}</div>
      <div style={{ fontSize: 12, marginTop: 6 }}>
        {owned.length === 0
          ? "No OLTs at this site"
          : `${online} of ${owned.length} OLTs online`}
      </div>
    </div>
  );
}
