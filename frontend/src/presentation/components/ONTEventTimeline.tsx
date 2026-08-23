import React, { useState } from "react";
import {
  Timeline,
  Typography,
  Empty,
  Spin,
  Tag,
  Card,
  Statistic,
  Row,
  Col,
  Select,
} from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
} from "@ant-design/icons";
import { useOntEvents, useOntAvailability } from "@/application/hooks";
import type { ONTEvent } from "@/domain/entities";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import duration from "dayjs/plugin/duration";

dayjs.extend(relativeTime);
dayjs.extend(duration);

const { Text, Title } = Typography;

interface ONTEventTimelineProps {
  ontId: string;
  days?: number;
}

const formatDuration = (seconds?: number) => {
  if (!seconds) return "N/A";

  const dur = dayjs.duration(seconds, "seconds");
  const days = Math.floor(dur.asDays());
  const hours = dur.hours();
  const minutes = dur.minutes();

  if (days > 0) {
    return `${days}d ${hours}h ${minutes}m`;
  }
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  return `${minutes}m`;
};

export const ONTEventTimeline: React.FC<ONTEventTimelineProps> = ({
  ontId,
  days: initialDays = 7,
}) => {
  const [selectedDays, setSelectedDays] = useState(initialDays);
  const { data: eventsData, isLoading: eventsLoading } = useOntEvents(
    ontId,
    100,
    0,
  );
  const { data: availabilityData, isLoading: availabilityLoading } =
    useOntAvailability(ontId, selectedDays);

  if (eventsLoading || availabilityLoading) {
    return (
      <div style={{ textAlign: "center", padding: "40px" }}>
        <Spin size="large" />
      </div>
    );
  }

  const events = eventsData?.events || [];

  const getEventIcon = (eventType: string) => {
    return eventType === "online" ? (
      <CheckCircleOutlined style={{ fontSize: "16px", color: "#52c41a" }} />
    ) : (
      <CloseCircleOutlined style={{ fontSize: "16px", color: "#ff4d4f" }} />
    );
  };

  const getEventColor = (eventType: string) => {
    return eventType === "online" ? "green" : "red";
  };

  const timelineItems = events.map((event: ONTEvent) => ({
    color: getEventColor(event.eventType),
    dot: getEventIcon(event.eventType),
    children: (
      <div>
        <div style={{ marginBottom: "4px" }}>
          <Tag color={getEventColor(event.eventType)}>
            {event.eventType ? event.eventType.toUpperCase() : "UNKNOWN"}
          </Tag>
          <Text type="secondary" style={{ fontSize: "12px" }}>
            {dayjs(event.eventTime).format("YYYY-MM-DD HH:mm:ss")}
          </Text>
          <Text
            type="secondary"
            style={{ fontSize: "12px", marginLeft: "8px" }}
          >
            ({dayjs(event.eventTime).fromNow()})
          </Text>
        </div>
        {event.reason && (
          <Text type="secondary" style={{ fontSize: "13px" }}>
            Reason: {event.reason}
          </Text>
        )}
        {event.durationSeconds && (
          <div style={{ marginTop: "4px" }}>
            <ClockCircleOutlined
              style={{ marginRight: "4px", fontSize: "12px" }}
            />
            <Text type="secondary" style={{ fontSize: "12px" }}>
              Duration: {formatDuration(event.durationSeconds)}
            </Text>
          </div>
        )}
      </div>
    ),
  }));

  return (
    <div>
      <div
        style={{
          marginBottom: "16px",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Title level={5} style={{ margin: 0 }}>
          Availability Statistics
        </Title>
        <Select
          value={selectedDays}
          onChange={setSelectedDays}
          style={{ width: 150 }}
          options={[
            { label: "Last 7 Days", value: 7 },
            { label: "Last 30 Days", value: 30 },
            { label: "Last 60 Days", value: 60 },
            { label: "Last 90 Days", value: 90 },
          ]}
        />
      </div>

      {availabilityData && (
        <Card style={{ marginBottom: "24px" }}>
          <Row gutter={16}>
            <Col span={6}>
              <Statistic
                title="Availability"
                value={availabilityData.availabilityPercent}
                precision={2}
                suffix="%"
                valueStyle={{
                  color:
                    availabilityData.availabilityPercent >= 99
                      ? "#3f8600"
                      : "#cf1322",
                }}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="Total Events"
                value={availabilityData.totalEvents}
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="MTBF (Mean Time Between Failures)"
                value={
                  availabilityData.mtbf > 0
                    ? formatDuration(availabilityData.mtbf)
                    : "N/A"
                }
              />
            </Col>
            <Col span={6}>
              <Statistic
                title="MTTR (Mean Time To Repair)"
                value={
                  availabilityData.mttr > 0
                    ? formatDuration(availabilityData.mttr)
                    : "N/A"
                }
              />
            </Col>
          </Row>
          <Row gutter={16} style={{ marginTop: "16px" }}>
            <Col span={8}>
              <Statistic
                title="Online Time"
                value={formatDuration(availabilityData.onlineSeconds)}
                valueStyle={{ color: "#3f8600" }}
              />
            </Col>
            <Col span={8}>
              <Statistic
                title="Offline Time"
                value={formatDuration(availabilityData.offlineSeconds)}
                valueStyle={{ color: "#cf1322" }}
              />
            </Col>
            <Col span={8}>
              <Statistic
                title="Total Time"
                value={formatDuration(availabilityData.totalSeconds)}
              />
            </Col>
          </Row>
        </Card>
      )}

      <Title level={5} style={{ marginBottom: "16px" }}>
        Event History
      </Title>

      {events.length === 0 ? (
        <Empty description="No events recorded yet" />
      ) : (
        <Timeline items={timelineItems} />
      )}
    </div>
  );
};
