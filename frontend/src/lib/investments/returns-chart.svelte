<script lang="ts">
  import type { ReturnsSeries } from '#lib/api/investments.ts';
  import { m } from '#lib/paraglide/messages.js';
  import { formatReturn, returnReasonText, returnsChartModel } from './returns-model';

  // R13 slice 3: the cumulative time-weighted series as a line. The plot
  // stretches to the panel width; labels are HTML so they stay legible on a
  // phone. The table of points is the accessible, exact alternative.
  let {
    series,
    locale,
    formatDate,
    missingCommodities
  }: {
    series: ReturnsSeries;
    locale: string;
    formatDate: (date: string) => string;
    /** Commodities only the chart points lacked a price or rate for. */
    missingCommodities: string[];
  } = $props();

  const chart = $derived(returnsChartModel(series.points));
  const first = $derived(series.points[0]);
  const last = $derived(series.points.at(-1));
  const width = 100;
  const height = 40;
  // A 5 % inset keeps the extremes off the frame.
  const plotY = (y: number) => height * (0.05 + 0.9 * y);
  const path = (segment: typeof chart.segments[number]) =>
    segment.map((point) => `${point.x * width},${plotY(point.y)}`).join(' ');
  const intervalText = $derived(
    series.interval === 'day' ? m.returns_chart_interval_day()
      : series.interval === 'week' ? m.returns_chart_interval_week()
        : m.returns_chart_interval_month()
  );
</script>

<figure class="space-y-2" data-testid="returns-chart">
  <figcaption>
    <span class="block text-sm font-semibold text-foreground">{m.returns_chart_title()}</span>
    <span class="mt-1 block text-xs text-muted">{m.returns_chart_help()} {intervalText}</span>
  </figcaption>

  {#if chart.available === 0}
    <p class="text-sm text-muted" data-testid="returns-chart-empty">{m.returns_chart_empty()}</p>
  {:else if first && last}
    <div class="flex gap-2">
      <div class="flex h-48 flex-col justify-between py-1 text-right text-xs tabular-nums text-muted" aria-hidden="true">
        <span>{formatReturn(chart.maximum.toFixed(6), locale)}</span>
        <span>{formatReturn(chart.minimum.toFixed(6), locale)}</span>
      </div>
      <div class="min-w-0 flex-1">
        <svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" class="h-48 w-full rounded-(--radius-control) border border-border bg-surface-strong/45"
          role="img" aria-label={m.returns_chart_accessible_label({ from: formatDate(first.date), to: formatDate(last.date) })}>
          <line x1="0" x2={width} y1={plotY(chart.zeroY)} y2={plotY(chart.zeroY)} stroke="var(--color-border)" stroke-width="1" stroke-dasharray="4 3" vector-effect="non-scaling-stroke" />
          {#each chart.segments as segment, index (index)}
            {#if segment.length === 1}
              <!-- A lone point between gaps: a round-capped zero-length stroke is a dot that survives the stretched viewBox. -->
              <line x1={segment[0].x * width} x2={segment[0].x * width} y1={plotY(segment[0].y)} y2={plotY(segment[0].y)}
                stroke="var(--color-accent)" stroke-width="6" stroke-linecap="round" vector-effect="non-scaling-stroke" />
            {:else}
              <polyline points={path(segment)} fill="none" stroke="var(--color-accent)" stroke-width="2.5" stroke-linejoin="round" vector-effect="non-scaling-stroke" />
            {/if}
          {/each}
        </svg>
        <div class="mt-1 flex justify-between text-xs text-muted" aria-hidden="true">
          <span>{formatDate(first.date)}</span>
          <span>{formatDate(last.date)}</span>
        </div>
      </div>
    </div>
  {/if}

  {#if chart.available < chart.total && chart.available > 0}
    <p class="text-sm text-warning" data-testid="returns-chart-gaps">{m.returns_chart_gaps({ missing: chart.total - chart.available, total: chart.total })}</p>
  {/if}
  {#if missingCommodities.length > 0}
    <p class="text-sm text-muted">{m.returns_chart_coverage({ commodities: missingCommodities.join(', ') })}</p>
  {/if}

  <details class="text-sm">
    <summary class="inline-flex min-h-11 cursor-pointer items-center font-medium text-foreground underline underline-offset-2">{m.returns_chart_data()}</summary>
    <!-- svelte-ignore a11y_no_noninteractive_tabindex (keyboard users must be able to scroll the bounded table) -->
    <div class="max-h-80 overflow-y-auto" role="region" tabindex="0" aria-label={m.returns_chart_data()}>
      <table class="w-full text-left" data-testid="returns-chart-table">
        <thead>
          <tr class="text-xs text-muted">
            <th scope="col" class="py-1 pr-3 font-medium">{m.returns_chart_column_date()}</th>
            <th scope="col" class="py-1 font-medium">{m.returns_chart_column_return()}</th>
          </tr>
        </thead>
        <tbody>
          {#each series.points as point (point.date)}
            <tr class="border-t border-border">
              <td class="py-1 pr-3 text-foreground">{formatDate(point.date)}</td>
              {#if point.time_weighted.value !== null}
                <td class="py-1 tabular-nums text-foreground">{formatReturn(point.time_weighted.value, locale)}</td>
              {:else}
                <td class="py-1 text-muted">{returnReasonText(point.time_weighted.reason, point.time_weighted.date, formatDate)}</td>
              {/if}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </details>
</figure>
