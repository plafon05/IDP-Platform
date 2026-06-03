import type { LucideIcon } from 'lucide-react';

type MetricCardProps = {
  icon: LucideIcon;
  label: string;
  value: string;
  tone: 'blue' | 'green' | 'orange' | 'violet';
};

export function MetricCard({ icon: Icon, label, value, tone }: MetricCardProps) {
  return (
    <article className={`metric-card ${tone}`}>
      <div className="metric-icon">
        <Icon size={20} aria-hidden="true" />
      </div>
      <div>
        <span>{label}</span>
        <strong>{value}</strong>
      </div>
    </article>
  );
}
