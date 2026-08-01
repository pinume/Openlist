import { SideMenuItemProps } from "./SideMenu"
import { BsPersonCircle, BsFingerprint } from "solid-icons/bs"
import { CgDatabase } from "solid-icons/cg"
import { IoHome } from "solid-icons/io"
import { Component, lazy } from "solid-js"
import { UserRole } from "~/types"

export type SideMenuItem = SideMenuItemProps & {
  component?: Component
  children?: SideMenuItem[]
}

export const side_menu_items: SideMenuItem[] = [
  {
    title: "manage.sidemenu.home",
    icon: IoHome,
    to: "/",
    role: UserRole.GENERAL,
    refresh: true,
  },
  {
    title: "manage.sidemenu.profile",
    icon: BsFingerprint,
    to: "/@manage",
    role: UserRole.GENERAL,
    component: lazy(() => import("./users/Profile")),
  },
  {
    title: "manage.sidemenu.users",
    icon: BsPersonCircle,
    to: "/@manage/users",
    component: lazy(() => import("./users/Users")),
  },
  {
    title: "manage.sidemenu.storages",
    icon: CgDatabase,
    to: "/@manage/storages",
    component: lazy(() => import("./storages/Storages")),
  },
]
