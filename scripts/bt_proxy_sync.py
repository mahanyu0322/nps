#!/www/server/panel/pyenv/bin/python3
import argparse
import json
import os
import sys
from urllib.parse import urlparse

if "/www/server/panel/class" not in sys.path:
    sys.path.insert(0, "/www/server/panel/class")
if "/www/server/panel" not in sys.path:
    sys.path.insert(0, "/www/server/panel")

os.chdir("/www/server/panel")

import public
from mod.project.proxy.comMod import main as ProxyMain


def response(status, msg, **extra):
    payload = {"status": bool(status), "msg": msg}
    payload.update(extra)
    return payload


def normalize_domain(raw):
    value = (raw or "").strip()
    if not value:
        raise ValueError("域名不能为空")
    if "://" in value:
        parsed = urlparse(value)
        value = parsed.netloc or parsed.path
    value = value.strip().strip("/")
    if "/" in value:
        value = value.split("/", 1)[0]
    if not value:
        raise ValueError("域名不能为空")
    host = value
    port = ""
    if host.count(":") == 1:
        host, port = host.rsplit(":", 1)
        if port and not port.isdigit():
            raise ValueError("域名端口不合法")
    host = host.strip().strip(".").lower()
    if not host:
        raise ValueError("域名不能为空")
    try:
        host = host.encode("idna").decode("ascii")
    except Exception:
        raise ValueError("域名格式不合法")
    return "{}:{}".format(host, port) if port else host


def expected_site_name(domain):
    host = domain.split(":", 1)[0]
    port = domain.split(":", 1)[1] if ":" in domain else "80"
    if port != "80":
        return "{}_{}".format(host, port)
    return host


def normalize_target_url(raw):
    target = (raw or "").strip()
    if not target.startswith("http://") and not target.startswith("https://"):
        raise ValueError("代理目标必须以 http:// 或 https:// 开头")
    if not target.endswith("/"):
        target += "/"
    return target


def get_site_info(site_name):
    if not site_name:
        return None
    return public.M("sites").where("name=? AND project_type=?", (site_name, "proxy")).field(
        "id,name,project_type"
    ).find()


def make_args(**kwargs):
    args = public.dict_obj()
    for key, value in kwargs.items():
        setattr(args, key, value)
    return args


def get_proxy_conf(proxy_obj, site_name):
    result = proxy_obj.get_global_conf(make_args(site_name=site_name))
    if not result.get("status"):
        raise RuntimeError(result.get("msg") or "读取宝塔反代配置失败")
    return result.get("data") or {}


def ensure_site(domain, target, site_name, remark):
    proxy_obj = ProxyMain()
    site_info = get_site_info(site_name)
    if not site_info:
        create_result = proxy_obj.create(
            make_args(
                domains=domain,
                proxy_path="/",
                proxy_pass=target,
                proxy_host="$http_host",
                proxy_type="http",
                remark=remark or "",
            )
        )
        if not create_result.get("status"):
            raise RuntimeError(create_result.get("msg") or "创建宝塔反代站点失败")
        site_name = expected_site_name(domain)
        site_info = get_site_info(site_name)
        if not site_info:
            raise RuntimeError("宝塔反代站点已创建，但未找到站点记录")

    conf = get_proxy_conf(proxy_obj, site_info["name"])
    current_domains = conf.get("domain_list") or []

    if domain not in current_domains:
        add_result = proxy_obj.add_domain(
            make_args(site_name=site_info["name"], id=site_info["id"], domains=domain)
        )
        if not add_result.get("status"):
            raise RuntimeError(add_result.get("msg") or "添加宝塔绑定域名失败")
        conf = get_proxy_conf(proxy_obj, site_info["name"])
        current_domains = conf.get("domain_list") or []

    remove_domains = [item for item in current_domains if item != domain]
    if remove_domains:
        delete_result = proxy_obj.batch_del_domain(
            make_args(
                site_name=site_info["name"],
                id=site_info["id"],
                domains="\n".join(remove_domains),
            )
        )
        if not delete_result.get("status"):
            raise RuntimeError(delete_result.get("msg") or "清理旧域名失败")

    update_result = proxy_obj.set_url_proxy(
        make_args(
            site_name=site_info["name"],
            proxy_path="/",
            proxy_host="$http_host",
            proxy_pass=target,
            proxy_type="http",
            remark=remark or "",
            websocket=1,
        )
    )
    if not update_result.get("status"):
        raise RuntimeError(update_result.get("msg") or "更新宝塔反代目标失败")

    return response(
        True,
        "ok",
        site_name=site_info["name"],
        site_id=site_info["id"],
        domain=domain,
        target=target,
    )


def delete_site(site_name):
    site_info = get_site_info(site_name)
    if not site_info:
        return response(True, "ok", site_name=site_name, deleted=False)

    proxy_obj = ProxyMain()
    delete_result = proxy_obj.delete(
        make_args(site_name=site_info["name"], id=site_info["id"], reload=1, remove_path=0)
    )
    if not delete_result.get("status"):
        raise RuntimeError(delete_result.get("msg") or "删除宝塔反代站点失败")
    return response(True, "ok", site_name=site_info["name"], deleted=True)


def main():
    parser = argparse.ArgumentParser(description="Sync BaoTa proxy site for nps localhost tunnels")
    subparsers = parser.add_subparsers(dest="action", required=True)

    upsert_parser = subparsers.add_parser("upsert")
    upsert_parser.add_argument("--domain", required=True)
    upsert_parser.add_argument("--target", required=True)
    upsert_parser.add_argument("--site-name", default="")
    upsert_parser.add_argument("--remark", default="")

    delete_parser = subparsers.add_parser("delete")
    delete_parser.add_argument("--site-name", required=True)

    args = parser.parse_args()

    try:
        if args.action == "upsert":
            domain = normalize_domain(args.domain)
            target = normalize_target_url(args.target)
            result = ensure_site(domain, target, (args.site_name or "").strip(), args.remark)
        else:
            result = delete_site((args.site_name or "").strip())
    except Exception as exc:
        result = response(False, str(exc))

    sys.stdout.write(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
