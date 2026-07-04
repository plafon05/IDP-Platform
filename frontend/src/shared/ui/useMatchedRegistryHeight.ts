import { useLayoutEffect, useRef } from 'react';

export function useMatchedRegistryHeight() {
  const layoutRef = useRef<HTMLElement>(null);

  useLayoutEffect(() => {
    const layout = layoutRef.current;
    if (!layout) return;

    const media = window.matchMedia('(max-width: 960px)');
    const update = () => {
      const registry = layout.querySelector<HTMLElement>('.registry-panel');
      const source = layout.querySelector<HTMLElement>('.registry-height-source');
      if (!registry) return;
      registry.style.height = media.matches || !source ? 'auto' : `${source.getBoundingClientRect().height}px`;
    };
    const observer = new ResizeObserver(update);
    const bindSource = () => {
      observer.disconnect();
      const source = layout.querySelector<HTMLElement>('.registry-height-source');
      if (source) observer.observe(source);
      update();
    };
    const mutations = new MutationObserver(bindSource);
    mutations.observe(layout, { childList: true, subtree: true });
    media.addEventListener('change', update);
    bindSource();

    return () => {
      observer.disconnect();
      mutations.disconnect();
      media.removeEventListener('change', update);
      layout.querySelector<HTMLElement>('.registry-panel')?.style.removeProperty('height');
    };
  }, []);

  return layoutRef;
}
