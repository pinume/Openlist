import { createSignal, JSX } from "solid-js"
import { me } from "~/store"
import { TaskNameAnalyzer } from "./Tasks"
import { useT } from "~/hooks"

export const getPath = (
  device: string,
  path: string,
  asLink: boolean = true,
): JSX.Element => {
  const fullPath = (device === "/" ? "" : device) + path
  const prefix = me().base_path === "/" ? "" : me().base_path
  const accessible = fullPath.startsWith(prefix)
  const [underline, setUnderline] = createSignal(false)
  return accessible && asLink ? (
    <a
      style={underline() ? "text-decoration: underline" : ""}
      onMouseOver={() => setUnderline(true)}
      onMouseOut={() => setUnderline(false)}
      href={fullPath.slice(prefix.length)}
    >
      {fullPath}
    </a>
  ) : (
    <p>{fullPath}</p>
  )
}

export const getDecompressNameAnalyzer = (): TaskNameAnalyzer => {
  const t = useT()
  return {
    regex:
      /^decompress \[(.+)]\((.*\/([^\/]+))\)\[(.+)] to \[(.+)]\((.+)\) with password <(.*)>$/,
    title: (matches) => matches[3],
    attrs: {
      [t(`tasks.attr.decompress.src`)]: (matches) =>
        getPath(matches[1], matches[2]),
      [t(`tasks.attr.decompress.dst`)]: (matches) =>
        getPath(matches[5], matches[6]),
      [t(`tasks.attr.decompress.inner`)]: (matches) => <p>{matches[4]}</p>,
      [t(`tasks.attr.decompress.password`)]: (matches) => <p>{matches[7]}</p>,
    },
  }
}

export const getDecompressUploadNameAnalyzer = (): TaskNameAnalyzer => {
  const t = useT()
  return {
    regex: /^upload (.+) to \[(.+)]\((.+)\)$/,
    title: (matches) => matches[1],
    attrs: {
      [t(`tasks.attr.decompress.dst`)]: (matches) =>
        getPath(matches[2], matches[3]),
    },
  }
}
