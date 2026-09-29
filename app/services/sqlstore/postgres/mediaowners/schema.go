package mediaowners

import (
	"fmt"
	"strings"

	"github.com/lib/pq"
)

func (owner Owner) KindSQL(alias string) string {
	if owner.Kind == "moderation" {
		return "'moderation:' || " + alias + ".content_type"
	}
	return pq.QuoteLiteral(owner.Kind) + "::text"
}

func (owner Owner) Fields() []string {
	fields := append([]string{}, owner.Keys...)
	fields = append(fields, owner.Markup...)
	return append(fields, owner.Extra...)
}

func (owner Owner) SourceSQL(alias string) string {
	var fields []string
	for _, field := range owner.Fields() {
		fields = append(fields, pq.QuoteLiteral(field), alias+"."+pq.QuoteIdentifier(field))
	}
	return "jsonb_build_object(" + strings.Join(fields, ", ") + ")"
}

func UpdateSQL() string {
	var schema strings.Builder
	var sources, scopes []string
	for _, owner := range All {
		sources = append(sources, fmt.Sprintf(
			"SELECT o.%s AS tenant_id, %s AS kind, o.%s AS owner_id,\n    %s AS title, %s AS url,\n    %s AS source\nFROM %s",
			pq.QuoteIdentifier(owner.Tenant), owner.KindSQL("o"), pq.QuoteIdentifier(owner.ID),
			owner.Title, owner.URL, owner.SourceSQL("o"), owner.From,
		))
		if owner.Scope != "" {
			scopes = append(scopes, fmt.Sprintf(
				"SELECT o.%s AS tenant_id, %s AS kind, o.%s AS owner_id, %s AS scope\nFROM %s",
				pq.QuoteIdentifier(owner.Tenant), owner.KindSQL("o"), pq.QuoteIdentifier(owner.ID), owner.Scope, owner.From,
			))
		}
	}

	schema.WriteString("CREATE OR REPLACE VIEW media_reference_sources AS\n" + strings.Join(sources, "\nUNION ALL\n") + ";\n\n")
	schema.WriteString("CREATE OR REPLACE VIEW media_reference_scopes AS\n" + strings.Join(scopes, "\nUNION ALL\n") + ";\n\n")
	var candidates []string
	for _, character := range ReferenceIntroducers {
		candidates = append(candidates, "strpos($1, "+pq.QuoteLiteral(string(character))+") > 0")
	}
	fmt.Fprintf(&schema, `CREATE OR REPLACE FUNCTION media_text_may_reference(text)
RETURNS boolean LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT COALESCE(%s, false);
$$;

`, strings.Join(candidates, "\n        OR "))
	schema.WriteString(`CREATE OR REPLACE FUNCTION check_media_reference_completion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM media_reference_changes WHERE transaction_id=txid_current()) THEN
        RAISE EXCEPTION 'Media reference changes must be completed before commit' USING ERRCODE='23514';
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS media_reference_completion ON media_reference_changes;
CREATE CONSTRAINT TRIGGER media_reference_completion
AFTER INSERT OR UPDATE ON media_reference_changes
DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_media_reference_completion();

`)
	for _, owner := range All {
		schema.WriteString(owner.CaptureSQL())
	}

	schema.WriteString("CREATE OR REPLACE FUNCTION lock_media_reference_owners(requested_tenant integer, requested_key text)\nRETURNS void LANGUAGE plpgsql AS $$\nBEGIN\n")
	for _, target := range All {
		var owners []string
		for _, source := range All {
			var ids []string
			if source.Kind == target.Kind {
				id := "o." + pq.QuoteIdentifier(source.ID)
				if source.Kind == "moderation" {
					id = "o.content_type, " + id
				}
				ids = append(ids, id)
			}
			for _, parent := range source.Parents {
				if parent.Kind == target.Kind {
					ids = append(ids, parent.ID)
				}
			}
			for _, id := range ids {
				owners = append(owners, fmt.Sprintf(
					"SELECT %s FROM %s\n        JOIN media_asset_refs r ON r.tenant_id=o.%s AND r.kind=%s AND r.owner_id=o.%s\n        WHERE r.tenant_id=requested_tenant AND r.key=requested_key",
					id, source.From, pq.QuoteIdentifier(source.Tenant), source.KindSQL("o"), pq.QuoteIdentifier(source.ID),
				))
			}
		}

		identity := pq.QuoteIdentifier(target.ID)
		if target.Kind == "moderation" {
			identity = "(content_type, content_id)"
		}
		fmt.Fprintf(&schema, "    PERFORM 1 FROM %s WHERE %s=requested_tenant AND %s IN (\n        %s\n    ) ORDER BY %s FOR UPDATE;\n\n",
			pq.QuoteIdentifier(target.Table), pq.QuoteIdentifier(target.Tenant), identity, strings.Join(owners, "\n        UNION\n        "), identity,
		)
	}
	schema.WriteString("END;\n$$;\n")
	schema.WriteString("\nCREATE OR REPLACE FUNCTION unlink_media_reference_fields(integer, text, boolean, boolean, boolean)\nRETURNS void LANGUAGE plpgsql AS $$\nBEGIN\n")
	for _, owner := range All {
		if owner.Unlink != "" {
			schema.WriteString("    " + owner.Unlink + ";\n\n")
		}
	}
	schema.WriteString("END;\n$$;\n")
	return schema.String()
}

func (owner Owner) CaptureSQL() string {
	function := pq.QuoteIdentifier("capture_media_" + owner.Kind)
	var updates, unchanged []string
	for _, field := range owner.Fields() {
		column := pq.QuoteIdentifier(field)
		updates = append(updates, column)
		unchanged = append(unchanged, "OLD."+column+" IS NOT DISTINCT FROM NEW."+column)
	}
	var candidates []string
	for _, field := range owner.Keys {
		candidates = append(candidates, "COALESCE(NEW."+pq.QuoteIdentifier(field)+"::text, '') <> ''")
	}
	for _, field := range owner.Markup {
		candidates = append(candidates, "media_text_may_reference(NEW."+pq.QuoteIdentifier(field)+"::text)")
	}
	noReferences := ""
	if len(owner.Extra) == 0 {
		noReferences = fmt.Sprintf(`    IF NOT (%s) THEN
        IF TG_OP='UPDATE' THEN
            PERFORM replace_media_references(jsonb_build_array(jsonb_build_object(
                'tenant_id', NEW.%s, 'kind', %s, 'owner_id', NEW.%s, 'refs', '[]'::jsonb
            )));
        END IF;
        RETURN NULL;
    END IF;

`, strings.Join(candidates, " OR "), pq.QuoteIdentifier(owner.Tenant), owner.KindSQL("NEW"), pq.QuoteIdentifier(owner.ID))
	}

	return fmt.Sprintf(`CREATE OR REPLACE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        DELETE FROM media_reference_changes
        WHERE transaction_id=txid_current() AND tenant_id=OLD.%s AND kind=%s AND owner_id=OLD.%s;
        DELETE FROM media_asset_refs
        WHERE tenant_id=OLD.%s AND kind=%s AND owner_id=OLD.%s;
        RETURN NULL;
    END IF;

    IF TG_OP='UPDATE' AND %s THEN
        RETURN NULL;
    END IF;

%s    INSERT INTO media_reference_changes(transaction_id,tenant_id,kind,owner_id,source)
    VALUES(txid_current(),NEW.%s,%s,NEW.%s,%s)
    ON CONFLICT(transaction_id,tenant_id,kind,owner_id) DO UPDATE SET source=EXCLUDED.source;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS %s ON %s;
CREATE TRIGGER %s AFTER INSERT OR DELETE OR UPDATE OF %s ON %s
FOR EACH ROW EXECUTE FUNCTION %s();

`, function,
		pq.QuoteIdentifier(owner.Tenant), owner.KindSQL("OLD"), pq.QuoteIdentifier(owner.ID),
		pq.QuoteIdentifier(owner.Tenant), owner.KindSQL("OLD"), pq.QuoteIdentifier(owner.ID),
		strings.Join(unchanged, " AND "), noReferences,
		pq.QuoteIdentifier(owner.Tenant), owner.KindSQL("NEW"), pq.QuoteIdentifier(owner.ID), owner.SourceSQL("NEW"),
		pq.QuoteIdentifier("media_"+owner.Kind+"_references"), pq.QuoteIdentifier(owner.Table),
		pq.QuoteIdentifier("media_"+owner.Kind+"_references"), strings.Join(updates, ", "), pq.QuoteIdentifier(owner.Table), function,
	)
}
