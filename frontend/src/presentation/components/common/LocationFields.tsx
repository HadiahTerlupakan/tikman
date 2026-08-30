import { Col, Form, Input, Row } from "antd";
import type { FormInstance } from "antd";
import { AddressAutocomplete } from "@/presentation/components/sites/AddressAutocomplete";
import { coordinateError } from "@/presentation/components/sites/siteCoordinates";

interface LocationFieldsProps {
  /** The form these fields belong to, so a resolved place can fill all three. */
  form: FormInstance;
  /** Field name carrying the address text. Sites call it location. */
  addressName?: string;
}

/**
 * Address plus latitude and longitude, shared by the site and OLT forms.
 *
 * Both forms need the same three fields with the same cross-field rule, and
 * the rule is subtle enough — both filled or both empty, each within range —
 * that a second copy would drift from this one without anybody noticing.
 */
export function LocationFields({
  form,
  addressName = "location",
}: LocationFieldsProps) {
  return (
    <>
      <Form.Item name={addressName} label="Address">
        <AddressAutocomplete
          onResolved={(place) => {
            form.setFieldsValue({
              [addressName]: place.address,
              latitude: place.latitude.toString(),
              longitude: place.longitude.toString(),
            });
            // The pair validator lives on latitude, so filling both fields
            // programmatically has to re-run it or a previous error lingers.
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
    </>
  );
}
