// src/utils/dates.js
export function getDaysLeft(deadline) {
  if (!deadline || deadline === "なし") return null;
  const today = new Date();
  const target = new Date(deadline);
  const diffTime = target - today;
  return Math.ceil(diffTime / (1000 * 60 * 60 * 24));
}

export function groupLabelByDate(timestamp, todayStart) {
  const t = typeof timestamp === 'number' ? timestamp : new Date(timestamp).getTime();
  if (t >= todayStart) return "今日";

  const weekAgo = todayStart - 7 * 24 * 60 * 60 * 1000;
  const monthAgo = todayStart - 30 * 24 * 60 * 60 * 1000;

  if (t >= weekAgo) return "1週間以内";
  if (t >= monthAgo) return "1ヶ月以内";
  return "それ以前";
}
