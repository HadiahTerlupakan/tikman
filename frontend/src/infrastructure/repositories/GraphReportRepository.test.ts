import { beforeEach, describe, expect, it, vi } from "vitest";
import { GraphReportRepository } from "./GraphReportRepository";

const get = vi.fn();
const post = vi.fn();

vi.mock("../http/apiClient", () => ({
  apiClient: {
    get: (...args: unknown[]) => get(...args),
    post: (...args: unknown[]) => post(...args),
  },
}));

const jsonBlob = (body: unknown) =>
  new Blob([JSON.stringify(body)], { type: "application/json" });

// FileReader, as jsdom's Blob has no text().
const readBlob = (blob: Blob) =>
  new Promise<string>((resolve) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.readAsText(blob);
  });

describe("GraphReportRepository.graph", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("asks for the port by device_id, if_name and label and returns the PNG", async () => {
    const png = new Blob(["png"], { type: "image/png" });
    get.mockResolvedValue({ status: 200, data: png });

    const blob = await new GraphReportRepository().graph(
      7,
      "ether 1/1",
      "Uplink",
    );

    expect(blob).toBe(png);
    const [url, config] = get.mock.calls[0];
    expect(url).toBe("/api/v1/graph-reports/librenms/graph");
    expect(config.params).toEqual({
      device_id: 7,
      if_name: "ether 1/1",
      label: "Uplink",
    });
    expect(config.responseType).toBe("blob");
  });

  it("reports the server's own reason, which arrives inside a blob", async () => {
    get.mockResolvedValue({
      status: 400,
      data: jsonBlob({ error: "LibreNMS belum diatur di Pengaturan" }),
    });

    await expect(
      new GraphReportRepository().graph(7, "sfp1", "A"),
    ).rejects.toThrow("LibreNMS belum diatur di Pengaturan");
  });

  it("falls back to the status when the error body is not JSON", async () => {
    get.mockResolvedValue({ status: 502, data: new Blob(["Bad Gateway"]) });

    await expect(
      new GraphReportRepository().graph(7, "sfp1", "A"),
    ).rejects.toThrow("HTTP 502");
  });
});

describe("GraphReportRepository.preview", () => {
  const input = {
    customerName: "PT Contoh",
    message: "",
    ports: [{ deviceId: 7, deviceName: "core", ifName: "sfp1", label: "A" }],
  };

  beforeEach(() => {
    post.mockReset();
  });

  it("posts the unsaved report and returns each image as a PNG with its caption", async () => {
    post.mockResolvedValue({
      status: 200,
      data: {
        images: [
          { image: btoa("first"), caption: "Halo\n\nPort: A, B" },
          { image: btoa("second"), caption: "Halo\n\nPort: C" },
        ],
      },
    });

    const images = await new GraphReportRepository().preview(input);

    expect(images.map(({ image }) => image.type)).toEqual([
      "image/png",
      "image/png",
    ]);
    expect(await readBlob(images[1].image)).toBe("second");
    expect(images[1].caption).toBe("Halo\n\nPort: C");
    const [url, body] = post.mock.calls[0];
    expect(url).toBe("/api/v1/graph-reports/preview");
    expect(body).toEqual(input);
  });

  it("reports why LibreNMS failed instead of a generic error", async () => {
    post.mockResolvedValue({
      status: 502,
      data: { error: "LibreNMS tidak menjawab" },
    });

    await expect(new GraphReportRepository().preview(input)).rejects.toThrow(
      "LibreNMS tidak menjawab",
    );
  });
});

describe("GraphReportRepository.create", () => {
  beforeEach(() => {
    post.mockReset();
  });

  // humps turns mentionAll and sendTime into mention_all and send_time on
  // the way out.
  it("sends the tag-everyone choice and send time with the report", async () => {
    post.mockResolvedValue({ status: 201, data: {} });
    const input = {
      waGroupId: "g1",
      customerName: "PT Contoh",
      message: "",
      enabled: true,
      mentionAll: true,
      notifyPortStatus: false,
      sendTime: "14:30",
      ports: [{ deviceId: 7, deviceName: "core", ifName: "sfp1", label: "A" }],
    };

    await new GraphReportRepository().create(input);

    const [url, body] = post.mock.calls[0];
    expect(url).toBe("/api/v1/graph-reports");
    expect(body.mentionAll).toBe(true);
    expect(body.sendTime).toBe("14:30");
  });
});

describe("GraphReportRepository.telegramChats", () => {
  beforeEach(() => {
    get.mockReset();
  });

  it("lists the chats the bot has seen", async () => {
    const chats = [
      {
        id: "t1",
        chatId: -100123,
        title: "NOC",
        type: "supergroup",
        active: true,
      },
    ];
    get.mockResolvedValue({ status: 200, data: { data: chats } });

    await expect(new GraphReportRepository().telegramChats()).resolves.toEqual(
      chats,
    );
    expect(get).toHaveBeenCalledWith("/api/v1/graph-reports/telegram/chats");
  });

  it("treats a null list as no chats", async () => {
    get.mockResolvedValue({ status: 200, data: { data: null } });

    await expect(new GraphReportRepository().telegramChats()).resolves.toEqual(
      [],
    );
  });
});
