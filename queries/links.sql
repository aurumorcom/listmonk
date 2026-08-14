-- links
-- name: create-link
INSERT INTO links (uuid, url) VALUES($1, $2) ON CONFLICT (url) DO UPDATE SET url=EXCLUDED.url RETURNING uuid;

-- name: get-link-url
SELECT url FROM links WHERE uuid = $1;

-- name: register-link-click
WITH link AS(
    SELECT id, url FROM links WHERE uuid = $1
)
INSERT INTO link_clicks (
    campaign_id, subscriber_id, link_id, ip_address, geo_country, geo_region, geo_city, geo_asn,
    user_agent, device_type, client_os, client_browser, email_client, is_bot, bot_type, sequence_step_id,
    variant_id, link_position, utm_params
) VALUES(
    (SELECT id FROM campaigns WHERE uuid = $2),
    (SELECT id FROM subscribers WHERE
        (CASE WHEN $3::TEXT != '' THEN subscribers.uuid = $3::UUID ELSE FALSE END)
    ),
    (SELECT id FROM link),
    NULLIF($4::TEXT, '')::INET,
    NULLIF($5::TEXT, ''),
    NULLIF($6::TEXT, ''),
    NULLIF($7::TEXT, ''),
    NULLIF($8::TEXT, ''),
    NULLIF($9::TEXT, ''),
    NULLIF($10::TEXT, ''),
    NULLIF($11::TEXT, ''),
    NULLIF($12::TEXT, ''),
    NULLIF($13::TEXT, ''),
    $14::BOOLEAN,
    NULLIF($15::TEXT, ''),
    $16::INTEGER,
    NULLIF($17::TEXT, ''),
    NULLIF($18::TEXT, ''),
    COALESCE($19::JSONB, '{}'::JSONB)
) RETURNING (SELECT url FROM link);
