import { Result, Button } from 'antd';
import { useNavigate } from 'react-router-dom';

export default function NotFoundPage() {
  const navigate = useNavigate();

  return (
    <Result
      status="404"
      title="404"
      subTitle="Halaman tidak ditemukan"
      extra={
        <Button type="primary" onClick={() => navigate('/')}>
          Kembali ke Dashboard
        </Button>
      }
    />
  );
}
