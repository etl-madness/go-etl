<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="3.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="xhtml" encoding="UTF-8" indent="yes" />

    <!-- Declare parameter received from Go -->
    <xsl:param name="diagram" select="''" />
    <xsl:param name="file" select="''" />
    <xsl:template match="/pipeline">
        <html lang="en" xmlns="http://www.w3.org/1999/xhtml">
            <head>
                <meta charset="utf-8"/>
                <title>ETL Pipeline Documentation &amp; Flowchart  <xsl:value-of select="$file" /> </title>
                <!-- Marked.js & Mermaid.js CDN Libraries -->
                <script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>
                <script src="https://cdn.jsdelivr.net/npm/mermaid@9/dist/mermaid.min.js"></script>
                <style>
                    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; margin: 30px; background-color: #f8f9fa; color: #333; }
                    h1, h2 { color: #1a252f; border-bottom: 2px solid #e9ecef; padding-bottom: 8px; }
                    .diagram-card { background: #ffffff; padding: 20px; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.05); margin-bottom: 30px; text-align: center; }
                    table { width: 100%; border-collapse: collapse; margin-bottom: 25px; background: #fff; }
                    th, td { text-align: left; padding: 10px 14px; border: 1px solid #dee2e6; }
                    th { background-color: #e9ecef; color: #495057; }
                    tr:nth-child(even) { background-color: #f8f9fa; }
                    code { background-color: #f1f3f5; padding: 2px 6px; border-radius: 4px; font-family: monospace; }
                </style>
            </head>
            <body>
                <h1>Pipeline Specification &amp; Flowchart <xsl:value-of select="$file" /></h1>

                <!-- 1. MERMAID DIAGRAM BLOCK -->
                <h2>Execution Flow Diagram</h2>
                <div class="diagram-card">
                    <div class="mermaid">
                        <xsl:value-of select="$diagram" />
                    </div>
                    <script>
                        mermaid.initialize({ startOnLoad: true });
                    </script>
                </div>
                <!-- 2. PIPELINE VARIABLES -->
                <xsl:if test="variables/variable">
                    <h2>Configured Variables</h2>
                    <table>
                        <thead>
                            <tr><th>Name</th><th>Type</th><th>Default Value</th></tr>
                        </thead>
                        <tbody>
                            <xsl:for-each select="variables/variable">
                                <tr>
                                    <td><strong><xsl:value-of select="@name"/></strong></td>
                                    <td><code><xsl:value-of select="@type"/></code></td>
                                    <td><code><xsl:value-of select="@value"/></code></td>
                                </tr>
                            </xsl:for-each>
                        </tbody>
                    </table>
                </xsl:if>

                <!-- 3. DATABASES -->
                <xsl:if test="databases/database">
                    <h2>Configured Databases</h2>
                    <table>
                        <thead>
                            <tr><th>Alias Name</th><th>Connection String / Variable Reference</th></tr>
                        </thead>
                        <tbody>
                            <xsl:for-each select="databases/database">
                                <tr>
                                    <td><strong><xsl:value-of select="@name"/></strong></td>
                                    <td><code><xsl:value-of select="@connection_string"/></code></td>
                                </tr>
                            </xsl:for-each>
                        </tbody>
                    </table>
                </xsl:if>
                <h2>SCRIPTS</h2>
                <table>
                    <thead>
                        <tr>
                            <th>Language</th>
                            <th>ID/Name</th>
                            <th>XPath Location</th>
                            <th>Source Database</th>
                            <th>Target Database</th>
                            <th>Target Table</th>
                            <th>Batch Size</th>
                            <th>Value</th>
                        </tr>
                    </thead>
                    <tbody>
                        <!-- Target matching nodes (e.g., all <script> nodes) -->
                        <xsl:for-each select="//script">
                            <tr>
         
                                <td><strong><xsl:value-of select="@language"/></strong></td>
                                <td><strong><xsl:value-of select="@id"/></strong></td>
                                <td><code><xsl:value-of select="path()"/></code></td>
                                <td><code><xsl:value-of select="@db"/></code></td>
                                <td>
                                    <code>
                                        <xsl:choose>
                                            <xsl:when test="@target_db and normalize-space(@target_db) != ''">
                                                <xsl:value-of select="@target_db"/>
                                            </xsl:when>
                                            <xsl:otherwise>
                                             <xsl:value-of select="@db"/>
                                            </xsl:otherwise>
                                        </xsl:choose>
                                    </code>
                                </td>

                                <td><code><xsl:value-of select="@target_table"/></code></td>
                                <td><code><xsl:value-of select="@batch_size"/></code></td>
                                <td><xsl:value-of select="."/></td>
                            </tr>
                        </xsl:for-each>
                    </tbody>
                </table>

            </body>
        </html>
    </xsl:template>

    <!-- ==================================================================== -->
    <!-- ENTRY & EXIT ID RESOLVERS (Fallback to generate-id if @id is missing)-->
    <!-- ==================================================================== -->
    <xsl:template match="script" mode="get-entry-id">
        <xsl:text>script_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="parallel" mode="get-entry-id">
        <xsl:text>par_start_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="child::if" mode="get-entry-id">
        <xsl:text>if_start_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="foreach|loop|while" mode="get-entry-id">
        <xsl:text>loop_start_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="group" mode="get-entry-id">
        <xsl:choose>
            <xsl:when test="*"><xsl:apply-templates select="*[1]" mode="get-entry-id"/></xsl:when>
            <xsl:otherwise><xsl:text>grp_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/></xsl:otherwise>
        </xsl:choose>
    </xsl:template>

    <xsl:template match="script" mode="get-exit-id">
        <xsl:text>script_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="parallel" mode="get-exit-id">
        <xsl:text>par_end_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="child::if" mode="get-exit-id">
        <xsl:text>if_end_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="foreach|loop|while" mode="get-exit-id">
        <xsl:text>loop_end_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/>
    </xsl:template>
    <xsl:template match="group" mode="get-exit-id">
        <xsl:choose>
            <xsl:when test="*"><xsl:apply-templates select="*[last()]" mode="get-exit-id"/></xsl:when>
            <xsl:otherwise><xsl:text>grp_</xsl:text><xsl:value-of select="if (@id) then @id else generate-id()"/></xsl:otherwise>
        </xsl:choose>
    </xsl:template>

    <!-- ==================================================================== -->
    <!-- MERMAID GENERATOR TEMPLATES                                           -->
    <!-- ==================================================================== -->

    <!-- SCRIPT NODE -->
    <xsl:template match="script" mode="render-mermaid">
        <xsl:variable name="nodeId" select="concat('script_', if (@id) then @id else generate-id())" />
        <xsl:variable name="scriptId">
            <xsl:choose>
                <xsl:when test="@id"><xsl:value-of select="@id"/></xsl:when>
                <xsl:otherwise>script_<xsl:value-of select="generate-id()"/></xsl:otherwise>
            </xsl:choose>
        </xsl:variable>
        <xsl:variable name="lang">
            <xsl:choose>
                <xsl:when test="@language"><xsl:value-of select="@language"/></xsl:when>
                <xsl:when test="@lang"><xsl:value-of select="@lang"/></xsl:when>
                <xsl:otherwise>SQL</xsl:otherwise>
            </xsl:choose>
        </xsl:variable>

    <xsl:value-of select="$nodeId"/>["<xsl:value-of select="$scriptId"/> &lt;br/&gt; (<xsl:value-of select="upper-case($lang)"/>)<xsl:if test="@target_db"> &lt;br/&gt; ➔ Stream to <xsl:value-of select="@target_db"/></xsl:if>"]
        <xsl:if test="following-sibling::*">
            <xsl:variable name="nextEntry"><xsl:apply-templates select="following-sibling::*[1]" mode="get-entry-id"/></xsl:variable>
    <xsl:value-of select="$nodeId"/> --&gt; <xsl:value-of select="$nextEntry"/>
        </xsl:if>
    </xsl:template>

    <!-- PARALLEL NODE -->
    <xsl:template match="parallel" mode="render-mermaid">
        <xsl:variable name="pStart" select="concat('par_start_', if (@id) then @id else generate-id())"/>
        <xsl:variable name="pEnd" select="concat('par_end_', if (@id) then @id else generate-id())"/>

    <xsl:value-of select="$pStart"/>{"⚡ Parallel Execution"}
    <xsl:value-of select="$pEnd"/>(( Join ))
        <xsl:apply-templates select="*" mode="render-mermaid"/>
        <xsl:for-each select="*">
            <xsl:variable name="cEntry"><xsl:apply-templates select="." mode="get-entry-id"/></xsl:variable>
            <xsl:variable name="cExit"><xsl:apply-templates select="." mode="get-exit-id"/></xsl:variable>
    <xsl:value-of select="$pStart"/> --&gt; <xsl:value-of select="$cEntry"/>
    <xsl:value-of select="$cExit"/> --&gt; <xsl:value-of select="$pEnd"/>
        </xsl:for-each>
        <xsl:if test="following-sibling::*">
            <xsl:variable name="nextEntry"><xsl:apply-templates select="following-sibling::*[1]" mode="get-entry-id"/></xsl:variable>
    <xsl:value-of select="$pEnd"/> --&gt; <xsl:value-of select="$nextEntry"/>
        </xsl:if>
    </xsl:template>

    <!-- IF NODE -->
    <xsl:template match="child::if" mode="render-mermaid">
        <xsl:variable name="ifStart" select="concat('if_start_', if (@id) then @id else generate-id())"/>
        <xsl:variable name="ifEnd" select="concat('if_end_', if (@id) then @id else generate-id())"/>
        <xsl:variable name="cond">
            <xsl:choose>
                <xsl:when test="@condition"><xsl:value-of select="@condition"/></xsl:when>
                <xsl:when test="@var"><xsl:value-of select="@var"/> == <xsl:value-of select="@equals"/></xsl:when>
                <xsl:otherwise>Check Condition</xsl:otherwise>
            </xsl:choose>
        </xsl:variable>

    <xsl:value-of select="$ifStart"/>{"❓ If: <xsl:value-of select="replace(translate($cond, '&quot;', &quot;'&quot;), '&quot;', &quot;'&quot;)"/>"}
    <xsl:value-of select="$ifEnd"/>(( Rejoin ))

        <!-- THEN BRANCH -->
        <xsl:choose>
            <xsl:when test="then/*">
                <xsl:apply-templates select="then/*" mode="render-mermaid"/>
                <xsl:variable name="thenEntry"><xsl:apply-templates select="then/*[1]" mode="get-entry-id"/></xsl:variable>
                <xsl:variable name="thenExit"><xsl:apply-templates select="then/*[last()]" mode="get-exit-id"/></xsl:variable>
    <xsl:value-of select="$ifStart"/> -- "Yes / Then" --&gt; <xsl:value-of select="$thenEntry"/>
    <xsl:value-of select="$thenExit"/> --&gt; <xsl:value-of select="$ifEnd"/>
            </xsl:when>
            <xsl:when test="*[not(self::then or self::else)]">
                <xsl:apply-templates select="*[not(self::then or self::else)]" mode="render-mermaid"/>
                <xsl:variable name="thenEntry"><xsl:apply-templates select="*[not(self::then or self::else)][1]" mode="get-entry-id"/></xsl:variable>
                <xsl:variable name="thenExit"><xsl:apply-templates select="*[not(self::then or self::else)][last()]" mode="get-exit-id"/></xsl:variable>
    <xsl:value-of select="$ifStart"/> -- "Yes / Then" --&gt; <xsl:value-of select="$thenEntry"/>
    <xsl:value-of select="$thenExit"/> --&gt; <xsl:value-of select="$ifEnd"/>
            </xsl:when>
            <xsl:otherwise>
    <xsl:value-of select="$ifStart"/> -- "Yes / Then" --&gt; <xsl:value-of select="$ifEnd"/>
            </xsl:otherwise>
        </xsl:choose>

        <!-- ELSE BRANCH -->
        <xsl:choose>
            <xsl:when test="else/*">
                <xsl:apply-templates select="else/*" mode="render-mermaid"/>
                <xsl:variable name="elseEntry"><xsl:apply-templates select="else/*[1]" mode="get-entry-id"/></xsl:variable>
                <xsl:variable name="elseExit"><xsl:apply-templates select="else/*[last()]" mode="get-exit-id"/></xsl:variable>
    <xsl:value-of select="$ifStart"/> -- "No / Else" --&gt; <xsl:value-of select="$elseEntry"/>
    <xsl:value-of select="$elseExit"/> --&gt; <xsl:value-of select="$ifEnd"/>
            </xsl:when>
            <xsl:otherwise>
    <xsl:value-of select="$ifStart"/> -- "No / Else" --&gt; <xsl:value-of select="$ifEnd"/>
            </xsl:otherwise>
        </xsl:choose>

        <xsl:if test="following-sibling::*">
            <xsl:variable name="nextEntry"><xsl:apply-templates select="following-sibling::*[1]" mode="get-entry-id"/></xsl:variable>
    <xsl:value-of select="$ifEnd"/> --&gt; <xsl:value-of select="$nextEntry"/>
        </xsl:if>
    </xsl:template>

    <!-- LOOP / FOREACH / WHILE NODE -->
    <xsl:template match="foreach|loop|while" mode="render-mermaid">
        <xsl:variable name="loopStart" select="concat('loop_start_', if (@id) then @id else generate-id())"/>
        <xsl:variable name="loopEnd" select="concat('loop_end_', if (@id) then @id else generate-id())"/>
        <xsl:variable name="loopName">
            <xsl:choose>
                <xsl:when test="@id"><xsl:value-of select="@id"/></xsl:when>
                <xsl:otherwise>Loop</xsl:otherwise>
            </xsl:choose>
        </xsl:variable>

    <xsl:value-of select="$loopStart"/>{"🔄 Loop: <xsl:value-of select="$loopName"/><xsl:if test="@var"> (<xsl:value-of select="@var"/>)</xsl:if>"}
    <xsl:value-of select="$loopEnd"/>(( Loop Exit ))

        <xsl:if test="*">
            <xsl:apply-templates select="*" mode="render-mermaid"/>
            <xsl:variable name="cEntry"><xsl:apply-templates select="*[1]" mode="get-entry-id"/></xsl:variable>
            <xsl:variable name="cExit"><xsl:apply-templates select="*[last()]" mode="get-exit-id"/></xsl:variable>
    <xsl:value-of select="$loopStart"/> -- "Next Row" --&gt; <xsl:value-of select="$cEntry"/>
    <xsl:value-of select="$cExit"/> --&gt; <xsl:value-of select="$loopStart"/>
        </xsl:if>
    <xsl:value-of select="$loopStart"/> -- "Done" --&gt; <xsl:value-of select="$loopEnd"/>

        <xsl:if test="following-sibling::*">
            <xsl:variable name="nextEntry"><xsl:apply-templates select="following-sibling::*[1]" mode="get-entry-id"/></xsl:variable>
    <xsl:value-of select="$loopEnd"/> --&gt; <xsl:value-of select="$nextEntry"/>
        </xsl:if>
    </xsl:template>

    <!-- GROUP NODE -->
    <xsl:template match="group" mode="render-mermaid">
        <xsl:apply-templates select="*" mode="render-mermaid"/>
        <xsl:if test="following-sibling::*">
            <xsl:variable name="groupExit"><xsl:apply-templates select="." mode="get-exit-id"/></xsl:variable>
            <xsl:variable name="nextEntry"><xsl:apply-templates select="following-sibling::*[1]" mode="get-entry-id"/></xsl:variable>
    <xsl:value-of select="$groupExit"/> --&gt; <xsl:value-of select="$nextEntry"/>
        </xsl:if>
    </xsl:template>

</xsl:stylesheet>