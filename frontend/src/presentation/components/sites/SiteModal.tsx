import { Modal, Form, Input, Row, Col } from "antd";
import {
  type Site,
  type CreateSiteDto,
  type UpdateSiteDto,
} from "@/domain/entities";
import { useEffect } from "react";
import { coordinateError, parseCoordinate } from "./siteCoordinates";
import { AddressAutocomplete } from "./AddressAutocomplete";

interface SiteModalProps {
  open: boolean;
  site?: Site;
  onClose: () => void;
  onSubmit: (data: CreateSiteDto | UpdateSiteDto) => void;
  loading: boolean;
}

interface SiteFormValues {
  name: string;
  location?: string;
  description?: string;
  latitude?: string;
  longitude?: string;
}

export function SiteModal({
  open,
  site,
  onClose,
  onSubmit,
  loading,
}: SiteModalProps) {
  const [form] = Form.useForm<SiteFormValues>();

  useEffect(() => {
    if (site) {
      form.setFieldsValue({
        name: site.name,
        location: site.location,
        description: site.description,
        latitude: site.latitude?.toString() ?? "",
        longitude: site.longitude?.toString() ?? "",
      });
    } else {
      form.resetFields();
    }
  }, [site, form]);

  const handleSubmit = () => {
    form
      .validateFields()
      .then((values) => {
        const latitude = parseCoordinate(values.latitude ?? "");
        const longitude = parseCoordinate(values.longitude ?? "");
        // Leaving both fields empty on a site that had a pin is a request to
        // remove it, and the API cannot tell an omitted coordinate from a null
        // one -- so say so rather than sending nothing and reporting success.
        const clearing =
          site !== undefined &&
          latitude === null &&
          longitude === null &&
          site.latitude !== undefined &&
          site.longitude !== undefined;

        onSubmit({
          name: values.name,
          location: values.location,
          description: values.description,
          ...(latitude !== null && longitude !== null
            ? { latitude, longitude }
            : {}),
          ...(clearing ? { clearCoordinates: true } : {}),
        });
      })
      // antd renders each failure against its own field, so there is nothing
      // left to report — but without this the rejection escapes unhandled.
      .catch(() => undefined);
  };

  return (
    <Modal
      title={site ? "Edit Site" : "Create Site"}
      open={open}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="name"
          label="Site Name"
          rules={[{ required: true, message: "Please enter site name" }]}
        >
          <Input />
        </Form.Item>

        {/* Form.Item clones its child and injects value/onChange, and those
            win over anything passed here — so the field is form-controlled and
            only onResolved is ours to supply. */}
        <Form.Item name="location" label="Address">
          <AddressAutocomplete
            onResolved={(place) => {
              form.setFieldsValue({
                location: place.address,
                latitude: place.latitude.toString(),
                longitude: place.longitude.toString(),
              });
              void form.validateFields(["latitude"]);
            }}
          />
        </Form.Item>

        <Row gutter={12}>
          <Col span={12}>
            <Form.Item
              name="latitude"
              label="Latitude"
              dependencies={["longitude"]}
              rules={[
                ({ getFieldValue }) => ({
                  validator: () => {
                    const error = coordinateError(
                      getFieldValue("latitude") ?? "",
                      getFieldValue("longitude") ?? "",
                    );
                    return error
                      ? Promise.reject(new Error(error))
                      : Promise.resolve();
                  },
                }),
              ]}
            >
              <Input placeholder="-6.4025" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="longitude" label="Longitude">
              <Input placeholder="106.7942" />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item name="description" label="Description">
          <Input.TextArea rows={4} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
